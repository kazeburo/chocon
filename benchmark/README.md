Benchmarks under (simulated) high latency network.

## Requirements

- Docker & Docker Compose (1.13.0+)

## Usage

```console
$ cat > config.json <<EOF
{
    "useHTTPS": [true],
    "latency": {
        "begin": 10,
        "end": 100,
        "step": 1.2,
        "mul": true
    },
    "useChocon": [true, false],
    "duration": {
        "begin": 30,
        "end": 30,
        "step": 1
    },
    "bodySize": {
        "begin": 100,
        "end": 100,
        "step": 1
    },
    "keepAlive": [true],
    "cpuLimit": {
        "begin": 0.25,
        "end": 2,
        "step": 0.25
    },
    "memoryLimit": {
        "begin": 256,
        "end": 2048,
        "step": 256
    }
}
EOF
$ cat config.json | go run main.go
useHTTPS bodySize latency useChocon duration concurrency keepAlive cpuLimit memoryLimit success fail
...
```