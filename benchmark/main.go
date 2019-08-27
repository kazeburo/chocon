package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
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

const eps = 1e-6

type bools []bool

func (b *bools) slice(defaultVal bool) []bool {
	if b == nil {
		return []bool{defaultVal}
	}

	return *b
}

type nums struct {
	Mul   bool
	Begin float64
	End   float64
	Step  float64
}

func (n *nums) slice(defaultVal float64) []float64 {
	ret := make([]float64, 0)

	if n == nil {
		return append(ret, defaultVal)
	}

	i := n.Begin

	for i < n.End+eps {
		ret = append(ret, i)

		if n.Mul {
			i *= n.Step
		} else {
			i += n.Step
		}
	}

	return ret
}

func main() {
	b, err := ioutil.ReadAll(os.Stdin)

	if err != nil {
		log.Fatal(err)
	}

	c := struct {
		BodySize    *nums
		Latency     *nums
		Duration    *nums
		MemoryLimit *nums
		CPULimit    *nums
		UseHTTPS    *bools
		UseChocon   *bools
		Concurrency *nums
		KeepAlive   *bools
	}{}

	if err := json.Unmarshal(b, &c); err != nil {
		log.Fatal(err)
	}

	fmt.Println(
		"useHTTPS", "bodySize", "latency", "useChocon", "duration",
		"concurrency", "keepAlive", "cpuLimit", "memoryLimit",
		"success", "fail",
	)

	for _, bodySize := range c.BodySize.slice(100) {
		for _, latency := range c.Latency.slice(100) {
			for _, duration := range c.Duration.slice(30) {
				for _, memoryLimit := range c.MemoryLimit.slice(2048) {
					for _, cpuLimit := range c.CPULimit.slice(1) {
						for _, useHTTPS := range c.UseHTTPS.slice(true) {
							for _, useChocon := range c.UseChocon.slice(true) {
								for _, concurrency := range c.Concurrency.slice(10) {
									for _, keepAlive := range c.KeepAlive.slice(true) {
										success, fail, err := run(
											useHTTPS, int(bodySize+eps), int(latency+eps), useChocon, int(duration+eps),
											// Round the float to its nearest int.
											int(concurrency+eps),
											keepAlive, 0, float32(cpuLimit), uint(memoryLimit+eps),
										)

										if err != nil {
											log.Fatal(err)
										}

										fmt.Println(
											useHTTPS, int(bodySize+eps), int(latency+eps), useChocon, int(duration+eps),
											int(concurrency+eps), keepAlive, 0, float32(cpuLimit), uint(memoryLimit+eps),
											success, fail,
										)
									}
								}
							}
						}
					}
				}
			}
		}
	}
}
