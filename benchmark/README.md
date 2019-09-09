Measure the performance of Chocon under simulated high latency network.

It sets up 3 Docker containers, one for an echo server, one for Chocon, and one for a http load generator. And then, we can see how many requests can be made it through between the client (load generator) and the server via Chocon.

## Notes

- Results include the number of successful responses (the throughput) and the average response time.
- The virtual network interface's qdisc of the server container is modified to simulate high latency network.
- It is designed to make it possible to test with multiple parameters at once using a configuration file. I.e. for each parameter, you can set a range of values, instead of one value. Refer to the "Usage" section below to get the idea.
- It is possible to limit the CPU/memory usage of the Chocon container. This is important so as to rule out the influence of the resource usage by the server conatiner and the client conatiner.

## Requirements

- Docker (>=19.03)

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
$ go run main.go < config.json
useHTTPS bodySize latency useChocon duration concurrency keepAlive cpuLimit memoryLimit success fail
...
```

- `{"begin": 1, "end": 8, "step": 2}` represents `[1, 3, 5, 7]`
- `{"begin": 1, "end": 8, "step": 2, "mul": true}` represents `[1, 2, 4, 8]`

### Parameters

- useHTTPS
    - If true, HTTPS is used between the server container and the Chocon container.
- latency
    - The latency between the client container and the server container can be set. If this is set to 10, the RTT between them will be 20ms.
- useChocon
    - If false, Chocon is not used for the benchmark. This is useful to see how much load can be generated.
- duration
    - The number of seconds for each test. If this is set to 10, each benchmark will last for 10 seconds.
- bodySize
    - The body size of each request (in bytes.)
- keepAlive
    - If true, the keep-alive is used for the connection between the load balancer and Chocon.
- cpuLimit
    - How much CPU resource can be used by the Chocon container. If this is set to 1.5, the Chocon container can use 1.5 cores out of the available CPU cores.
- memoryLimit
    - How much memory can be used by the Chocon container. If this is set to 2048, the Chocon container can use up to 2GB of memory.
