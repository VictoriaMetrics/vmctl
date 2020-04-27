# vmctl - Victoria metrics command-line tool

Features:
- [x] Prometheus: migrate data from Prometheus to VictoriaMetrics using snapshot API
- [ ] ~~Prometheus: migrate data from Prometheus to VictoriaMetrics by query~~(discarded)
- [x] InfluxDB: migrate data from InfluxDB to VictoriaMetrics
- [ ] Storage Management: data re-balancing between nodes 

# How to build

1. [Install Go](https://golang.org/doc/install). The minimum supported version is Go 1.12.
2. Run `make build` from the root folder of the repository.
   It builds `vmctl` binary and puts it into the `bin` folder.
   
## Migrating data from InfluxDB

`vmctl` supports the `influx` mode to migrate data from InfluxDB to VictoriaMetrics time-series database.
See `help` for details:
```
./vmctl influx --help
NAME:
   vmctl influx - Migrate timeseries from InfluxDB

USAGE:
   vmctl influx [command options] [arguments...]

OPTIONS:
   --influx-addr value              Influx server addr (default: "http://localhost:8086")
   --influx-user value              Influx user [$INFLUX_USERNAME]
   --influx-password value          Influx user password [$INFLUX_PASSWORD]
   --influx-database value          Influx database
   --influx-retention-policy value  Influx retention policy (default: "autogen")
   --influx-chunk-size value        The chunkSize defines max amount of series to be returned in one chunk (default: 10000)
   --influx-concurrency value       Number of concurrently running fetch queries to InfluxDB (default: 1)
   --influx-filter-series value     Influx filter expression to select series. E.g. "from cpu where arch='x86' AND hostname='host_2753'".
See for details https://docs.influxdata.com/influxdb/v1.7/query_language/schema_exploration#show-series
   --influx-filter-time-start value            The time filter to select timeseries with timestamp equal or higher than provided value. E.g. '2020-01-01T20:07:00Z'
   --influx-filter-time-end value              The time filter to select timeseries with timestamp equal or lower than provided value. E.g. '2020-01-01T20:07:00Z'
   --influx-measurement-field-separator value  The {separator} symbol used to concatenate {measurement} and {field} names into series name {measurement}{separator}{field}. (default: "_")
   --vm-addr value                             VictoriaMetrics address to perform import requests. Should be the same as --httpListenAddr value for single-node version or VMSelect component. (default: "http://localhost:8428")
   --vm-user value                             VictoriaMetrics username for basic auth [$VM_USERNAME]
   --vm-password value                         VictoriaMetrics password for basic auth [$VM_PASSWORD]
   --vm-account-id value                       Account(tenant) ID - is required for cluster VM. (default: -1)
   --vm-concurrency value                      Number of workers concurrently performing import requests to VM (default: 2)
   --vm-compress                               Whether to apply gzip compression to import requests (default: true)
   --vm-batch-size value                       How many datapoints importer collects before sending the import request to VM (default: 200000)
   --help, -h                                  show help (default: false)
```

To use migration tool please specify the InfluxDB address `--influx-addr`, the database `--influx-database` and VictoriaMetrics address `--vm-addr`.
Flag `--vm-addr` for single-node VM is usually equal to `--httpListenAddr`, and for cluster version
is equal to `--httpListenAddr` flag of VMInsert component. Please note that for cluster version it is required to specify
the `--vm-account-id` flag as well. See more details for cluster version [here](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/cluster).

As soon as required flags are provided and all endpoints are accessible, `vmctl` will start the InfluxDB scheme exploration.
Basically, it just fetches all fields and timeseries from the provided database and builds up registry of all available timeseries.
Then `vmctl` sends fetch requests for each timeseries to InfluxDB one by one and pass results to VM importer.
VM importer then accumulates received datapoints in batches and sends import requests to VM.

The importing process example for local installation of InfluxDB(`http://localhost:8086`) 
and single-node VictoriaMetrics(`http://localhost:8428`):
```
./vmctl influx --influx-database benchmark
InfluxDB import mode
2020/01/18 20:47:11 Exploring scheme for database "benchmark"
2020/01/18 20:47:11 fetching fields: command: "show field keys"; database: "benchmark"; retention: "autogen"
2020/01/18 20:47:11 found 10 fields
2020/01/18 20:47:11 fetching series: command: "show series "; database: "benchmark"; retention: "autogen"
Found 40000 timeseries to import. Continue? [Y/n] y
40000 / 40000 [-----------------------------------------------------------------------------------------------------------------------------------------------] 100.00% 21 p/s
2020/01/18 21:19:00 Import finished!
2020/01/18 21:19:00 VictoriaMetrics importer stats:
  time spent while waiting: 13m51.461434876s;
  time spent while importing: 17m56.923899847s;
  total datapoints: 345600000;
  datapoints/s: 320914.04;
  total bytes: 5.9 GB;
  bytes/s: 5.4 MB;
  import requests: 40001;
2020/01/18 21:19:00 Total time: 31m48.467044016s
``` 

### Data mapping

Vmctl maps Influx data the same way as VictoriaMetrics does by using the following rules:

* `influx-database` arg is mapped into `db` label value unless `db` tag exists in the Influx line.
* Field names are mapped to time series names prefixed with {measurement}{separator} value, 
where {separator} equals to _ by default. 
It can be changed with `--influx-measurement-field-separator` command-line flag.
* Field values are mapped to time series values.
* Tags are mapped to Prometheus labels format as-is.

For example, the following Influx line:
```
foo,tag1=value1,tag2=value2 field1=12,field2=40
```

is converted into the following Prometheus format data points:
```
foo_field1{tag1="value1", tag2="value2"} 12
foo_field2{tag1="value1", tag2="value2"} 40
```

### Configuration

The configuration flags should contain self-explanatory descriptions. 

#### Filtering

The filtering consists of two parts: timeseries and time.
The first step of application is to select all available timeseries
for given database and retention. User may specify additional filtering
condition via `--influx-filter-series` flag. For example:
```
./vmctl influx --influx-database benchmark --influx-filter-series "on benchmark from cpu where hostname='host_1703'"
InfluxDB import mode
2020/01/26 14:23:29 Exploring scheme for database "benchmark"
2020/01/26 14:23:29 fetching fields: command: "show field keys"; database: "benchmark"; retention: "autogen"
2020/01/26 14:23:29 found 12 fields
2020/01/26 14:23:29 fetching series: command: "show series on benchmark from cpu where hostname='host_1703'"; database: "benchmark"; retention: "autogen"
Found 10 timeseries to import. Continue? [Y/n] 
```
The timeseries select query would be following:
 `fetching series: command: "show series on benchmark from cpu where hostname='host_1703'"; database: "benchmark"; retention: "autogen"`
 
The second step of filtering is a time filter and it applies when fetching the datapoints from Influx.
Time filtering may be configured with two flags:
* --influx-filter-time-start 
* --influx-filter-time-end 
Here's an example of importing timeseries for one day only:
`./vmctl influx --influx-database benchmark --influx-filter-series "where hostname='host_1703'" --influx-filter-time-start "2020-01-01T10:07:00Z" --influx-filter-time-end "2020-01-01T15:07:00Z"`

Please see more about time filtering [here](https://docs.influxdata.com/influxdb/v1.7/query_language/schema_exploration#filter-meta-queries-by-time).


## Migrating data from Prometheus

`vmctl` supports the `prometheus` mode for migrating data from Prometheus to VictoriaMetrics time-series database.
Migration is based on reading Prometheus snapshot, which is basically a hard-link to Prometheus data files.
Thanos uses the same storage engine as Prometheus and the data layout on-disk should be the same. That means
`vmctl` may be used for Thanos historical data migration as well.

See `help` for details:
```
./vmctl prometheus --help
NAME:
   vmctl prometheus - Migrate timeseries from Prometheus

USAGE:
   vmctl prometheus [command options] [arguments...]

OPTIONS:
   --prom-snapshot value            Path to Prometheus snapshot. Pls see for details https://www.robustperception.io/taking-snapshots-of-prometheus-data
   --prom-concurrency value         Number of concurrently running snapshot readers (default: 1)
   --prom-filter-time-start value   The time filter to select timeseries with timestamp equal or higher than provided value. E.g. '2020-01-01T20:07:00Z'
   --prom-filter-time-end value     The time filter to select timeseries with timestamp equal or lower than provided value. E.g. '2020-01-01T20:07:00Z'
   --prom-filter-label value        Prometheus label name to filter timeseries by. E.g. '__name__' will filter timeseries by name.
   --prom-filter-label-value value  Prometheus regular expression to filter label from "prom-filter-label" flag. (default: ".*")
   --vm-addr value                  VictoriaMetrics address to perform import requests. Should be the same as --httpListenAddr value for single-node version or VMSelect component. (default: "http://localhost:8428")
   --vm-user value                  VictoriaMetrics username for basic auth [$VM_USERNAME]
   --vm-password value              VictoriaMetrics password for basic auth [$VM_PASSWORD]
   --vm-account-id value            Account(tenant) ID - is required for cluster VM. (default: -1)
   --vm-concurrency value           Number of workers concurrently performing import requests to VM (default: 2)
   --vm-compress                    Whether to apply gzip compression to import requests (default: true)
   --vm-batch-size value            How many datapoints importer collects before sending the import request to VM (default: 200000)
   --help, -h                       show help (default: false)
```

To use migration tool please specify the path to Prometheus snapshot `--prom-snapshot` and VictoriaMetrics address `--vm-addr`.
More about Prometheus snapshots may be found [here](https://www.robustperception.io/taking-snapshots-of-prometheus-data).
Flag `--vm-addr` for single-node VM is usually equal to `--httpListenAddr`, and for cluster version
is equal to `--httpListenAddr` flag of VMInsert component. Please note that for cluster version it is required to specify
the `--vm-account-id` flag as well. See more details for cluster version [here](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/cluster).

As soon as required flags are provided and all endpoints are accessible, `vmctl` will start the Prometheus snapshot exploration.
Basically, it just fetches all available blocks in provided snapshot and read the metadata. It also does initial filtering by time
if flags `--prom-filter-time-start` or `--prom-filter-time-end` were set. The exploration procedure prints some stats from read blocks.
Please note that stats are not taking into account timeseries or samples filtering. This will be done during importing process.
 
The importing process takes the snapshot blocks revealed from Explore procedure and processes them one by one
accumulating timeseries and datapoints. The data processed in chunks and then sent to VM. 

The importing process example for local installation of Prometheus 
and single-node VictoriaMetrics(`http://localhost:8428`):
```
./vmctl prometheus --prom-snapshot=/path/to/snapshot --vm-concurrency 1 --vm-batch-size=200000 --prom-concurrency 3
Prometheus import mode
Prometheus snapshot stats:
  blocks found: 14;
  blocks skipped: 0;
  min time: 1581288163058 (2020-02-09T22:42:43Z);
  max time: 1582409128139 (2020-02-22T22:05:28Z);
  samples: 32549106;
  series: 27289.
Filter is not taken into account for series and samples numbers.
Found 14 blocks to import. Continue? [Y/n] y
14 / 14 [-------------------------------------------------------------------------------------------] 100.00% 0 p/s
2020/02/23 15:50:03 Import finished!
2020/02/23 15:50:03 VictoriaMetrics importer stats:
  time spent while waiting: 6.152953029s;
  time spent while importing: 44.908522491s;
  total datapoints: 32549106;
  datapoints/s: 724786.84;
  total bytes: 669.1 MB;
  bytes/s: 14.9 MB;
  import requests: 323;
  import requests retries: 0;
2020/02/23 15:50:03 Total time: 51.077451066s
``` 

### Data mapping

VictoriaMetrics has very similar data model to Prometheus and supports [RemoteWrite integration](https://prometheus.io/docs/operating/integrations/#remote-endpoints-and-storage).
So no data changes will be applied.

### Configuration

The configuration flags should contain self-explanatory descriptions. 

#### Filtering

The filtering consists of three parts: by timeseries and time.

Filtering by time may be configured with flags `--prom-filter-time-start` and `--prom-filter-time-end`.
This filter applied twice: to drop blocks out of range and to filter timeseries in blocks with
overlapping time range.

Example of applying time filter:
```
./vmctl prometheus --prom-snapshot=/path/to/snapshot --prom-filter-time-start=2020-02-07T00:07:01Z --prom-filter-time-end=2020-02-11T00:07:01Z
Prometheus import mode
Prometheus snapshot stats:
  blocks found: 2;
  blocks skipped: 12;
  min time: 1581288163058 (2020-02-09T22:42:43Z);
  max time: 1581328800000 (2020-02-10T10:00:00Z);
  samples: 1657698;
  series: 3930.
Filter is not taken into account for series and samples numbers.
Found 2 blocks to import. Continue? [Y/n] y
```

Please notice, that total amount of blocks in provided snapshot is 14, but only 2 of them were in provided
time range. So other 12 blocks were marked as `skipped`. The amount of samples and series is not taken into account,
since this is heavy operation and will be done during import process.


Filtering by timeseries is configured with following flags: 
* `--prom-filter-label` - the label name, e.g. `__name__` or `instance`;
* `--prom-filter-label-value` - the regular expression to filter the label value. By default matches all `.*`

For example:
```
./vmctl prometheus --prom-snapshot=/path/to/snapshot --prom-filter-label="__name__" --prom-filter-label-value="promhttp.*" --prom-filter-time-start=2020-02-07T00:07:01Z --prom-filter-time-end=2020-02-11T00:07:01Z
Prometheus import mode
Prometheus snapshot stats:
  blocks found: 2;
  blocks skipped: 12;
  min time: 1581288163058 (2020-02-09T22:42:43Z);
  max time: 1581328800000 (2020-02-10T10:00:00Z);
  samples: 1657698;
  series: 3930.
Filter is not taken into account for series and samples numbers.
Found 2 blocks to import. Continue? [Y/n] y
14 / 14 [------------------------------------------------------------------------------------------------------------------------------------------------------] 100.00% ? p/s
2020/02/23 15:51:07 Import finished!
2020/02/23 15:51:07 VictoriaMetrics importer stats:
  time spent while waiting: 0s;
  time spent while importing: 37.415461ms;
  total datapoints: 10128;
  datapoints/s: 270690.24;
  total bytes: 195.2 kB;
  bytes/s: 5.2 MB;
  import requests: 2;
  import requests retries: 0;
2020/02/23 15:51:07 Total time: 7.153158218s
```

## Tuning

### Influx mode

The flag `--influx-concurrency` controls how many concurrent requests may be sent to InfluxDB while fetching
timeseries. Please set it wisely to avoid InfluxDB overwhelming.

The flag `--influx-chunk-size` controls the max amount of datapoints to return in single chunk from fetch requests.
Please see more details [here](https://docs.influxdata.com/influxdb/v1.7/guides/querying_data/#chunking).
The chunk size is used to control InfluxDB memory usage, so it won't OOM on processing large timeseries with 
billions of datapoints.

### Prometheus mode

The flag `--prom-concurrency` controls how many concurrent readers will be reading the blocks in snapshot.
Since snapshots are just files on disk it would be hard to overwhelm the system. Please go with value equal
to number of free CPU cores.

### VictoriaMetrics importer

The flag `--vm-concurrency` controls the number of concurrent workers that process the input from InfluxDB query results.
Please note that each import request can load up to a single vCPU core on VictoriaMetrics. So try to set it according
to allocated CPU resources of your VictoriMetrics installation.

The flag `--vm-batch-size` controls max amount of datapoints collected before sending the import request.
For example, if  `--influx-chunk-size=500` and `--vm-batch-size=2000` then importer will process not more 
than 4 chunks before sending the request. 

### Importer stats

After successful import `vmctl` prints some statistics for details. 
The important numbers to watch are following:
 - `time spent while waiting` - how much time importer spent while waiting for data from
 InfluxDB/Prometheus and grouping it into batches. This value may tell if InfluxDB fetches 
 were slow which probably may be improved by increasing `--<mode>-concurrency`.
 - `time spent while importing` - how much time importer spent while serializing data
 and executing import requests. The high number comparing to `time spent while waiting`
 may be a sign of VM being overloaded by import requests or other clients.
- `import requests` - shows how many import requests were issued to VM server.
The import request is issued once the batch size(`--vm-batch-size`) is full and ready to be sent.
Please prefer big batch sizes (50k-500k) to improve performance.

  
