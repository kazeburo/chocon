package main

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/kazeburo/chocon/benchmark/docker"
)

func run(useHTTPS bool, bodySize int, latency int, useChocon bool, duration int, concurrency int, keepAlive bool, n int) (int, int, error) {
	var compose *docker.Compose

	if useChocon {
		compose = docker.New("chocon", "server")
	} else {
		compose = docker.New("client", "server")
	}

	// Get the containers running.
	if err := compose.Up(true); err != nil {
		return 0, 0, err
	}

	// Make sure the containers get shut down.
	defer func() {
		compose.Down()
	}()

	var client *docker.Container
	if useChocon {
		client, _ = compose.Container("chocon")
	} else {
		client, _ = compose.Container("client")
	}

	// Add network latencies between containers.
	for _, container := range compose.Containers {
		if _, _, err := container.Execute(
			"tc", "qdisc", "add", "dev", "eth0", "root", "netem", "delay",
			fmt.Sprintf("%dms", latency),
		); err != nil {
			return 0, 0, err
		}
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

		args = append(args, "http://localhost:3000/")
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
	duration := 60

	for i := 0; i < 100; i++ {
		for _, useHTTPS := range []bool{true, false} {
			for _, useChocon := range []bool{true, false} {
				for concurrency := 1; concurrency <= 2048; concurrency *= 2 {
					for _, keepAlive := range []bool{true, false} {
						success, fail, err := run(useHTTPS, bodySize, latency, useChocon, duration, concurrency,
							keepAlive, 0)

						if err != nil {
							log.Fatal(err)
						}

						fmt.Println(useHTTPS, bodySize, latency, useChocon, duration, concurrency, keepAlive, success, fail)
					}
				}
			}
		}
	}
}
