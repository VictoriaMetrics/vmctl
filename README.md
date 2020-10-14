# vmctl - Victoria metrics command-line tool

Features:
- [x] Prometheus: migrate data from Prometheus to VictoriaMetrics using snapshot API
- [x] Thanos: migrate data from Thanos to VictoriaMetrics
- [ ] ~~Prometheus: migrate data from Prometheus to VictoriaMetrics by query~~(discarded)
- [x] InfluxDB: migrate data from InfluxDB to VictoriaMetrics
- [ ] Storage Management: data re-balancing between nodes 

# Table of contents

* [Articles](#articles)
* [How to build](#how-to-build)
* [Migrating data from InfluxDB](#migrating-data-from-influxdb)
   * [Data mapping](#data-mapping)
   * [Configuration](#configuration)
   * [Filtering](#filtering)
* [Migrating data from Prometheus](#migrating-data-from-prometheus)
   * [Data mapping](#data-mapping-1)
   * [Configuration](#configuration-1)
   * [Filtering](#filtering-1)
* [Migrating data from Thanos](#migrating-data-from-thanos)
   * [Current data](#current-data)
   * [Historical data](#historical-data)
* [Migrating data from VictoriaMetrics](#migrating-data-from-victoriametrix)
   * [Native protocol](#native-protocol)
* [Tuning](#tuning)
   * [Influx mode](#influx-mode)
   * [Prometheus mode](#prometheus-mode)
   * [VictoriaMetrics importer](#victoriametrics-importer)
   * [Importer stats](#importer-stats)


## Articles

* [How to migrate data from Prometheus](https://medium.com/@romanhavronenko/victoriametrics-how-to-migrate-data-from-prometheus-d44a6728f043)
* [How to migrate data from Prometheus. Filtering and modifying time series](https://medium.com/@romanhavronenko/victoriametrics-how-to-migrate-data-from-prometheus-filtering-and-modifying-time-series-6d40cea4bf21)

## How to build

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
   --vm-addr vmctl                  VictoriaMetrics address to perform import requests. 
Should be the same as --httpListenAddr value for single-node version or VMInsert component. 
Please note, that vmctl performs initial readiness check for the given address by checking `/health` endpoint. (default: "http://localhost:8428")
   --vm-user value                             VictoriaMetrics username for basic auth [$VM_USERNAME]
   --vm-password value                         VictoriaMetrics password for basic auth [$VM_PASSWORD]
   --vm-account-id value                       AccountID is an arbitrary 32-bit integer identifying namespace for data ingestion (aka tenant). 
It is possible to set it as accountID:projectID, where projectID is also arbitrary 32-bit integer. 
If projectID isn't set, then it equals to 0
   --vm-concurrency value                      Number of workers concurrently performing import requests to VM (default: 2)
   --vm-compress                               Whether to apply gzip compression to import requests (default: true)
   --vm-batch-size value                       How many samples importer collects before sending the import request to VM (default: 200000)
   --vm-significant-figures value              The number of significant figures to leave in metric values before importing. 
See https://en.wikipedia.org/wiki/Significant_figures. Zero value saves all the significant decimal places. 
This option may be used for increasing on-disk compression level for the stored metrics (default: 0)
   --help, -h                                  show help (default: false)
```

To use migration tool please specify the InfluxDB address `--influx-addr`, the database `--influx-database` and VictoriaMetrics address `--vm-addr`.
Flag `--vm-addr` for single-node VM is usually equal to `--httpListenAddr`, and for cluster version
is equal to `--httpListenAddr` flag of VMInsert component. Please note, that vmctl performs initial readiness check for the given address 
by checking `/health` endpoint. For cluster version it is additionally required to specify the `--vm-account-id` flag. 
See more details for cluster version [here](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/cluster).

As soon as required flags are provided and all endpoints are accessible, `vmctl` will start the InfluxDB scheme exploration.
Basically, it just fetches all fields and timeseries from the provided database and builds up registry of all available timeseries.
Then `vmctl` sends fetch requests for each timeseries to InfluxDB one by one and pass results to VM importer.
VM importer then accumulates received samples in batches and sends import requests to VM.

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
  idle duration: 13m51.461434876s;
  time spent while importing: 17m56.923899847s;
  total samples: 345600000;
  samples/s: 320914.04;
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

### Filtering

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
   --prom-filter-time-start value   The time filter in RFC3339 format to select timeseries with timestamp equal or higher than provided value. E.g. '2020-01-01T20:07:00Z'
   --prom-filter-time-end value     The time filter in RFC3339 format to select timeseries with timestamp equal or lower than provided value. E.g. '2020-01-01T20:07:00Z'
   --prom-filter-label value        Prometheus label name to filter timeseries by. E.g. '__name__' will filter timeseries by name.
   --prom-filter-label-value value  Prometheus regular expression to filter label from "prom-filter-label" flag. (default: ".*")
   --vm-addr vmctl                  VictoriaMetrics address to perform import requests. 
Should be the same as --httpListenAddr value for single-node version or VMInsert component. 
Please note, that vmctl performs initial readiness check for the given address by checking `/health` endpoint. (default: "http://localhost:8428")
   --vm-user value                  VictoriaMetrics username for basic auth [$VM_USERNAME]
   --vm-password value              VictoriaMetrics password for basic auth [$VM_PASSWORD]
   --vm-account-id value                       AccountID is an arbitrary 32-bit integer identifying namespace for data ingestion (aka tenant). 
It is possible to set it as accountID:projectID, where projectID is also arbitrary 32-bit integer. 
If projectID isn't set, then it equals to 0
   --vm-concurrency value           Number of workers concurrently performing import requests to VM (default: 2)
   --vm-compress                    Whether to apply gzip compression to import requests (default: true)
   --vm-batch-size value            How many samples importer collects before sending the import request to VM (default: 200000)
   --vm-significant-figures value   The number of significant figures to leave in metric values before importing. 
See https://en.wikipedia.org/wiki/Significant_figures. Zero value saves all the significant decimal places. 
This option may be used for increasing on-disk compression level for the stored metrics (default: 0)
   --help, -h                       show help (default: false)
```

To use migration tool please specify the path to Prometheus snapshot `--prom-snapshot` and VictoriaMetrics address `--vm-addr`.
More about Prometheus snapshots may be found [here](https://www.robustperception.io/taking-snapshots-of-prometheus-data).
Flag `--vm-addr` for single-node VM is usually equal to `--httpListenAddr`, and for cluster version
is equal to `--httpListenAddr` flag of VMInsert component. Please note, that vmctl performs initial readiness check for the given address 
by checking `/health` endpoint. For cluster version it is additionally required to specify the `--vm-account-id` flag. 
See more details for cluster version [here](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/cluster).

As soon as required flags are provided and all endpoints are accessible, `vmctl` will start the Prometheus snapshot exploration.
Basically, it just fetches all available blocks in provided snapshot and read the metadata. It also does initial filtering by time
if flags `--prom-filter-time-start` or `--prom-filter-time-end` were set. The exploration procedure prints some stats from read blocks.
Please note that stats are not taking into account timeseries or samples filtering. This will be done during importing process.
 
The importing process takes the snapshot blocks revealed from Explore procedure and processes them one by one
accumulating timeseries and samples. The data processed in chunks and then sent to VM. 

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
Found 14 blocks to import. Continue? [Y/n] y
14 / 14 [-------------------------------------------------------------------------------------------] 100.00% 0 p/s
2020/02/23 15:50:03 Import finished!
2020/02/23 15:50:03 VictoriaMetrics importer stats:
  idle duration: 6.152953029s;
  time spent while importing: 44.908522491s;
  total samples: 32549106;
  samples/s: 724786.84;
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

### Filtering

The filtering consists of three parts: by timeseries and time.

Filtering by time may be configured via flags `--prom-filter-time-start` and `--prom-filter-time-end`
in in RFC3339 format. This filter applied twice: to drop blocks out of range and to filter timeseries in blocks with
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
Found 2 blocks to import. Continue? [Y/n] y
14 / 14 [------------------------------------------------------------------------------------------------------------------------------------------------------] 100.00% ? p/s
2020/02/23 15:51:07 Import finished!
2020/02/23 15:51:07 VictoriaMetrics importer stats:
  idle duration: 0s;
  time spent while importing: 37.415461ms;
  total samples: 10128;
  samples/s: 270690.24;
  total bytes: 195.2 kB;
  bytes/s: 5.2 MB;
  import requests: 2;
  import requests retries: 0;
2020/02/23 15:51:07 Total time: 7.153158218s
```

## Migrating data from Thanos

Thanos uses the same storage engine as Prometheus and the data layout on-disk should be the same. That means
`vmctl` in mode `prometheus` may be used for Thanos historical data migration as well.
These instructions may vary based on the details of your Thanos configuration. 
Please read carefully and verify as you go. We assume you're using Thanos Sidecar on your Prometheus pods, 
and that you have a separate Thanos Store installation.

### Current data

1. For now, keep your Thanos Sidecar and Thanos-related Prometheus configuration, but add this to also stream 
    metrics to VictoriaMetrics:
    ```
    remote_write:
    - url: http://victoria-metrics:8428/api/v1/write
    ```
2. Make sure VM is running, of course. Now check the logs to make sure that Prometheus is sending and VM is receiving. 
    In Prometheus, make sure there are no errors. On the VM side, you should see messages like this:
    ```
    2020-04-27T18:38:46.474Z	info	VictoriaMetrics/lib/storage/partition.go:207	creating a partition "2020_04" with smallPartsPath="/victoria-metrics-data/data/small/2020_04", bigPartsPath="/victoria-metrics-data/data/big/2020_04"
    2020-04-27T18:38:46.506Z	info	VictoriaMetrics/lib/storage/partition.go:222	partition "2020_04" has been created
    ```
3. Now just wait. Within two hours, Prometheus should finish its current data file and hand it off to Thanos Store for long term
    storage.

### Historical data

Let's assume your data is stored on S3 served by minio. You first need to copy that out to a local filesystem, 
then import it into VM using `vmctl` in `prometheus` mode.
1. Copy data from minio.
    1. Run the `minio/mc` Docker container.
    1. `mc config host add minio http://minio:9000 accessKey secretKey`, substituting appropriate values for the last 3 items.
    1. `mc cp -r minio/prometheus thanos-data`
1. Import using `vmctl`.
    1. Follow the [instructions](#how-to-build) to compile `vmctl` on your machine.
    1. Use [prometheus](#migrating-data-from-prometheus) mode to import data: 
    ```
    vmctl prometheus --prom-snapshot thanos-data --vm-addr http://victoria-metrics:8428
    ```

## Migrating data from VictoriaMetrics

### Native protocol

The [native binary protocol](https://victoriametrics.github.io/#how-to-export-data-in-native-format)
was introduced in [1.42.0 release](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.42.0)
and provides the most efficient way to migrate data between VM instances: single to single, 
single to cluster and vice versa. Please note that both instances (source and destination) should be of v1.42.0
or higher.

See `help` for details:
```
 ./vmctl vm-native --help
NAME:
   vmctl vm-native - Migrate time series between VictoriaMetrics installations via native binary format

USAGE:
   vmctl vm-native [command options] [arguments...]

OPTIONS:
   --vm-native-filter-match value  Time series selector to match series for export. For example, select {instance!="localhost"} will match all series with "instance" label different to "localhost".
 See more details here https://github.com/VictoriaMetrics/VictoriaMetrics#how-to-export-data-in-native-format (default: "{__name__!=\"\"}")
   --vm-native-filter-time-start value  The time filter may contain either unix timestamp in seconds or RFC3339 values. E.g. '2020-01-01T20:07:00Z'
   --vm-native-filter-time-end value    The time filter may contain either unix timestamp in seconds or RFC3339 values. E.g. '2020-01-01T20:07:00Z'
   --vm-native-src-addr value           VictoriaMetrics address to perform export from. 
 Should be the same as --httpListenAddr value for single-node version or VMSelect component. If exporting from cluster version - include the tenet token in address.
   --vm-native-src-user value      VictoriaMetrics username for basic auth [$VM_NATIVE_SRC_USERNAME]
   --vm-native-src-password value  VictoriaMetrics password for basic auth [$VM_NATIVE_SRC_PASSWORD]
   --vm-native-dst-addr value      VictoriaMetrics address to perform import to. 
 Should be the same as --httpListenAddr value for single-node version or VMInsert component. If importing into cluster version - include the tenet token in address.
   --vm-native-dst-user value      VictoriaMetrics username for basic auth [$VM_NATIVE_DST_USERNAME]
   --vm-native-dst-password value  VictoriaMetrics password for basic auth [$VM_NATIVE_DST_PASSWORD]
```

In this mode `vmctl` acts as a proxy between two VM instances, where time series filtering is done by "source" (`src`) 
and processing is done by "destination" (`dst`). Because of that, `vmctl` doesn't actually know how much data will be 
processed and can't show the progress bar. It will show the current processing speed and total number of processed bytes:

```
./vmctl vm-native --vm-native-src-addr=http://localhost:8528  --vm-native-dst-addr=http://localhost:8428 --vm-native-filter-match='{job="vmagent"}'
VictoriaMetrics Native import mode
Initing export pipe from "http://localhost:8528" with filters: 
        filter: match[]={job="vmagent"}
Initing import process to "http://localhost:8428":
Total: 336.75 KiB ↖ Speed: 454.46 KiB p/s                                                                                                               
2020/10/13 17:04:59 Total time: 952.143376ms
``` 

Importing tips:
1. Migrating all the metrics from one VM to another may collide with existing application metrics 
(prefixed with `vm_`) at destination and lead to confusion when using 
[official Grafana dashboards](https://grafana.com/orgs/victoriametrics/dashboards). 
To avoid such situation try to filter out VM process metrics via `--vm-native-filter-match` flag.
2. Migration is a backfilling process, so it is recommended to read 
[Backfilling tips](https://github.com/VictoriaMetrics/VictoriaMetrics#backfilling) section.
3. `vmctl` doesn't provide relabeling or other types of labels management in this mode.
Instead, use [relabeling in VictoriaMetrics](https://github.com/VictoriaMetrics/vmctl/issues/4#issuecomment-683424375).


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

The flag `--vm-batch-size` controls max amount of samples collected before sending the import request.
For example, if  `--influx-chunk-size=500` and `--vm-batch-size=2000` then importer will process not more 
than 4 chunks before sending the request. 

### Importer stats

After successful import `vmctl` prints some statistics for details. 
The important numbers to watch are following:
 - `idle duration` - shows time that importer spent while waiting for data from InfluxDB/Prometheus 
to fill up `--vm-batch-size` batch size. Value shows total duration across all workers configured
via `--vm-concurrency`. High value may be a sign of too slow InfluxDB/Prometheus fetches or too
high `--vm-concurrency` value. Try to improve it by increasing `--<mode>-concurrency` value or 
decreasing `--vm-concurrency` value.
- `import requests` - shows how many import requests were issued to VM server.
The import request is issued once the batch size(`--vm-batch-size`) is full and ready to be sent.
Please prefer big batch sizes (50k-500k) to improve performance.
- `import requests retries` - shows number of unsuccessful import requests. Non-zero value may be
a sign of network issues or VM being overloaded. See the logs during import for error messages.

### Silent mode

By default `vmctl` waits confirmation from user before starting the import. If this is unwanted
behavior and no user interaction required - pass `-s` flag to enable "silence" mode:
```
    -s Whether to run in silent mode. If set to true no confirmation prompts will appear. (default: false)
```

### Significant figures

`vmctl` allows to limit the number of [significant figures](https://en.wikipedia.org/wiki/Significant_figures)
before importing. For example, the average value for response size is `102.342305` bytes and it has 9 significant figures.
If you ask a human to pronounce this value then with high probability value will be rounded to first 4 or 5 figures 
because the rest aren't really that important to mention. In most cases, such a high precision is too much. 
Moreover, such values may be just a result of [floating point arithmetic](https://en.wikipedia.org/wiki/Floating-point_arithmetic), 
create a [false precision](https://en.wikipedia.org/wiki/False_precision) and result into bad compression ratio 
according to [information theory](https://en.wikipedia.org/wiki/Information_theory). 

The `--vm-significant-figures` flag allows to limit the number of significant figures. It takes no effect if set 
to 0 (by default), but set `--vm-significant-figures=5` and `102.342305` will be rounded to `102.34`. Such value will 
have much higher compression ratio comparing to previous one and will save some extra disk space after the migration. 
The most common case for using this flag is to reduce number of significant figures for time series storing aggregation 
results such as `average`, `rate`, etc.   