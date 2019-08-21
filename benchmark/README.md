Benchmarks under (simulated) high latency network.

## Requirements

- Docker & Docker Compose (1.13.0+)

## Usage

```console
$ <cd to this directory>
$ ./run.sh 2> /dev/null
client -> [10ms latency] -> server (http)
first request
Time per request:       66.165 [ms] (mean)
following 100 requests
Time per request:       44.589 [ms] (mean)

client -> chocon -> [10ms latency] -> server (http)
first request
Time per request:       70.805 [ms] (mean)
following 100 requests
Time per request:       23.704 [ms] (mean)

client -> [10ms latency] -> server (https)
first request
Time per request:       87.447 [ms] (mean)
following 100 requests
Time per request:       92.394 [ms] (mean)

client -> chocon -> [10ms latency] -> server (https)
first request
Time per request:       97.595 [ms] (mean)
following 100 requests
Time per request:       24.403 [ms] (mean)
```
