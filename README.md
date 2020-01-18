# vmctl - Victoria metrics command-line tool

Features:
- [ ] Prometheus: migrate data from Prometheus to VictoriaMetrics using snapshot API
- [ ] Prometheus: migrate data from Prometheus to VictoriaMetrics by query
- [x] InfluxDB: migrate data from InfluxDB to VictoriaMetrics
- [ ] Storage Management: data re-balancing between nodes 

## Migrating data from InfluxDB

### How to build

1. [Install Go](https://golang.org/doc/install). The minimum supported version is Go 1.12.
2. Run `make build` from the root folder of the repository.
   It builds `vmctl` binary and puts it into the `bin` folder.

### How to use

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
   --influx-series-filter value     Influx filter expression to select timeseries. E.g. "arch='x64' AND host='host101'"
   --influx-chunk-size value        The chunkSize defines max amount of series to be returned in one chunk (default: 10000)
   --influx-concurrency value       Number of concurrently running fetch queries to InfluxDB (default: 1)
   --vm-addr value                  VictoriaMetrics address to perform import requests. Should be the same as --httpListenAddr value for single-node version or VMSelect component. (default: "http://localhost:8428")
   --vm-account-id value            Account(tenant) ID - for the cluster VM only (default: -1)
   --vm-concurrency value           Number of workers concurrently performing import requests to VM (default: 2)
   --vm-compress                    Whether to apply gzip compression to import requests (default: true)
   --vm-batch-size value            How many datapoints importer collects before sending the import request to VM (default: 200000)
   --help, -h                       show help (default: false)
```

To use migration tool please specify the InfluxDB address `--influx-addr`, the database `--influx-database` and VictoriaMetrics address `--vm-addr`.
Flag `--vm-addr` for single-node VM is usually equal to `--httpListenAddr`, and for cluster version
is equal to `--httpListenAddr` flag of VMInsert component. Please note that for cluster version it is required to sepcify
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

### Configuration

The configuration flags should contain self-explanatory descriptions. 

#### Filtering

In order to export only part of timeseries from InfluxDB please specify the `--influx-series-filter` flag.
It's value will be added to InfluxDB queries then:
```
./vmctl influx --influx-database benchmark --influx-series-filter "arch='x64' and datacenter='ap-northeast-1a' and time >= '2020-01-01T20:07:00Z' and time < '2020-01-01T21:07:00Z'"
InfluxDB import mode
2020/01/18 22:36:41 Exploring scheme for database "benchmark"
2020/01/18 22:36:41 fetching fields: command: "show field keys"; database: "benchmark"; retention: "autogen"
2020/01/18 22:36:41 found 10 fields
2020/01/18 22:36:41 fetching series: command: "show series where arch='x64' and datacenter='ap-northeast-1a' and time >= '2020-01-01T20:07:00Z' and time < '2020-01-01T21:07:00Z' "; database: "benchmark"; retention: "autogen"
Found 1350 timeseries to import. Continue? [Y/n] y
```

#### Performance

There are two components that may be configured in order to improve performance.

##### InfluxDB

The flag `--influx-concurrency` controls how many concurrent requests may be sent to InfluxDB while fetching
timeseries. Please set it wisely to avoid InfluxDB overwhelming.

The flag `--influx-chunk-size` controls the max amount of datapoints to return in single chunk from fetch requests.
Please see more details [here](https://docs.influxdata.com/influxdb/v1.7/guides/querying_data/#chunking).
The chunk size is used to control InfluxDB memory usage, so it won't OOM on processing large timeseries with 
billions of datapoints.

##### VictoriaMetrics importer

The flag `--vm-concurrency` controls the number of concurrent workers that process the input from InfluxDB query results.
Please note that each import request can load up to a single vCPU core on VictoriaMetrics. So try to set it according
to allocated CPU resources of your VictoriMetrics installation.

The flag `--vm-batch-size` controls max amount of datapoints collected before sending the import request.
For example, if  `--influx-chunk-size=500` and `--vm-batch-size=2000` then importer will process not more 
than 4 chunks before sending the request. 

#### Importer stats

After successful import `vmctl` prints some statistics for details. 
The important numbers to watch are following:
 - `time spent while waiting` - how much time importer spent while waiting for data from
 InfluxDB and grouping it into batches. This value may tell if InfluxDB fetches were slow
 which probably may be improved by increasing `--influx-concurrency`
 - `time spent while importing` - how much time importer spent while serializing data
 and executing import requests. The high number comparing to `time spent while waiting`
 may be a sign of VM being overloaded by import requests or other clients.
 