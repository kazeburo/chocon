package main

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"log"
	"math"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/kazeburo/chocon/benchmark/docker"
)

// Set up the environment (containers) and run the benchmark using `hey` HTTP load generator.
func run(
	useHTTPS bool,
	bodySize int,
	// If this is set to 10, inter-container network latency will be 10ms
	// i.e. the RTT between two containers will be 20ms.
	latency int,
	useChocon bool,
	// Number of seconds to send requests.
	duration int,
	// Number of concurrent requests to be made.
	concurrency int,
	// If this is true, TCP connections are reused between
	// hey and chocon (if useChocon is true), or hey and server
	// (if useChocon is false).
	keepAlive bool,
	// Number of requests to make.
	// This is ignored when `duration` is set.
	n int,
	// Set this to 0.5 so as to limit the chocon's cpu usage to 50%.
	cpuLimit float32,
	// Set this to 1024 so as to limit the chocon's memory usage to 1GB.
	memoryLimit uint,
) (int, int, error) {
	containers := []*docker.Container{
		{
			Name: "server",
			Build: struct {
				Context    string
				Dockerfile string
			}{"./server", ""},
			CapAdd: []string{"NET_ADMIN"},
		},
		{
			Name: "client",
			Build: struct {
				Context    string
				Dockerfile string
			}{"./client", ""},
			// Without this, the container gets shut down as soon as
			// it has been created.
			Tty:    true,
			CapAdd: []string{"NET_ADMIN"},
		},
	}

	if useChocon {
		containers = append(containers, &docker.Container{
			Name: "chocon",
			Build: struct {
				Context    string
				Dockerfile string
			}{"../.", "./benchmark/chocon/Dockerfile"},
			CapAdd:   []string{"NET_ADMIN"},
			Cpus:     cpuLimit,
			MemLimit: memoryLimit,
		})
	}

	compose := docker.New(containers...)

	// Get the containers running.
	if err := compose.Up(true, true); err != nil {
		return 0, 0, err
	}

	// Make sure the containers get shut down.
	defer func() {
		compose.Down()
	}()

	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		compose.Down()
		os.Exit(1)
	}()

	client, _ := compose.Container("client")
	server, _ := compose.Container("server")

	// Add network latencies between containers.
	if _, _, err := server.Execute(
		"tc", "qdisc", "add", "dev", "eth0", "root", "netem", "delay",
		fmt.Sprintf("%dms", latency*2),
	); err != nil {
		return 0, 0, err
	}

	// Make a dummy file.
	client.Execute("bash", "-c", fmt.Sprintf("yes | head -c %d > dummy", bodySize))

	// Wait until the servers are ready.
	time.Sleep(3 * time.Second)

	args := []string{
		"-disable-compression",
		"-D", "./dummy",
		"-c", strconv.Itoa(concurrency),
		"-o", "csv",
	}

	if duration != 0 {
		args = append(args, []string{"-z", strconv.Itoa(duration) + "s"}...)
	}

	if n != 0 {
		args = append(args, []string{"-n", strconv.Itoa(n)}...)
	}

	if !keepAlive {
		args = append(args, "-disable-keepalive")
	}

	if useChocon {
		args = append(args, "-host")

		if useHTTPS {
			args = append(args, "server.ccnproxy-secure.local")
		} else {
			args = append(args, "server.ccnproxy.local")
		}

		args = append(args, "http://chocon/")
	} else {
		if useHTTPS {
			args = append(args, "https://server/")
		} else {
			args = append(args, "http://server/")
		}
	}

	stdout, _, err := client.Execute("/root/go/bin/hey", args...)

	if err != nil {
		return 0, 0, err
	}

	records, err := csv.NewReader(bytes.NewReader(stdout)).ReadAll()

	if err != nil {
		return 0, 0, err
	}

	// The number of responses with 200 status code.
	success := 0
	// The number of responses with non-200 status code.
	fail := 0

	keys := records[0]

	for _, record := range records[1:] {
		for i, key := range keys {
			if key == "status-code" {
				if record[i] == "200" {
					success++
				} else {
					fail++
				}
			}
		}
	}

	return success, fail, nil
}

func main() {
	bodySize := 100
	latency := 10
	duration := 30

	fmt.Println("useHTTPS", "bodySize", "latency", "useChocon", "duration", "concurrency", "keepAlive", "cpuLimit", "memoryLimit", "success", "fail")

	for memoryLimit := uint(256); memoryLimit <= 4*1024; memoryLimit += 256 {
		for cpuLimit := float32(0.5); cpuLimit <= 2; cpuLimit += 0.25 {
			for _, useHTTPS := range []bool{true, false} {
				for _, useChocon := range []bool{true, false} {
					for concurrency := float64(1); concurrency <= 2048; concurrency *= math.Sqrt2 {
						for _, keepAlive := range []bool{true, false} {
							if !useChocon && memoryLimit != 256 && cpuLimit != 0.5 {
								continue
							}

							success, fail, err := run(
								useHTTPS,
								bodySize,
								latency,
								useChocon,
								duration,
								// Round the float to its nearest int.
								int(concurrency+0.5),
								keepAlive,
								0,
								cpuLimit,
								memoryLimit,
							)

							if err != nil {
								log.Fatal(err)
							}

							fmt.Println(useHTTPS, bodySize, latency, useChocon, duration, int(concurrency+0.5), keepAlive, cpuLimit, memoryLimit, success, fail)
						}
					}
				}
			}
		}
	}
}
