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
) (int, int, float64, error) {
	network, err := docker.CreateNetwork("chocon_benchmark")

	if err != nil {
		return 0, 0, 0, err
	}

	defer network.Remove()

	serverImg, err := docker.BuildImage("./server", "")

	if err != nil {
		return 0, 0, 0, err
	}

	clientImg, err := docker.BuildImage("./client", "")

	if err != nil {
		return 0, 0, 0, err
	}

	choconImg, err := docker.BuildImage("../.", "./chocon/Dockerfile")

	if err != nil {
		return 0, 0, 0, err
	}

	serverEnvironment := []struct {
		Key   string
		Value string
	}{}

	if useHTTPS {
		serverEnvironment = append(serverEnvironment, struct {
			Key   string
			Value string
		}{"HTTPS", "1"})
	}

	serverContainer, err := serverImg.Run(&docker.RunConfig{
		Network:      &network,
		Environments: serverEnvironment,
		Tty:          true,
		CapAdds:      []string{"NET_ADMIN"},
	})

	if err != nil {
		return 0, 0, 0, err
	}

	defer serverContainer.Stop()

	clientContainer, err := clientImg.Run(&docker.RunConfig{
		Network: &network,
		Tty:     true,
		CapAdds: []string{"NET_ADMIN"},
	})

	if err != nil {
		return 0, 0, 0, err
	}

	defer clientContainer.Stop()

	var choconContainer docker.Container

	if useChocon {
		choconContainer, err = choconImg.Run(&docker.RunConfig{
			Network:  &network,
			Tty:      true,
			CapAdds:  []string{"NET_ADMIN"},
			Cpus:     cpuLimit,
			MemLimit: memoryLimit,
		})

		if err != nil {
			return 0, 0, 0, err
		}

		defer choconContainer.Stop()
	}

	// Add network latencies between containers.
	if _, _, err := serverContainer.Execute(
		"tc", "qdisc", "add", "dev", "eth0", "root", "netem", "delay",
		fmt.Sprintf("%dms", latency*2),
	); err != nil {
		return 0, 0, 0, err
	}

	// Make a dummy file.
	clientContainer.Execute("bash", "-c", fmt.Sprintf("yes | head -c %d > dummy", bodySize))

	// Wait until the servers are ready.
	time.Sleep(3 * time.Second)

	args := []string{
		"-disable-compression",
		"-D", "./dummy",
		"-c", strconv.Itoa(concurrency),
		"-o", "csv",
		"-m", "POST",
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
			args = append(args, fmt.Sprintf("%s.ccnproxy-secure.local", serverContainer.ID[:12]))
		} else {
			args = append(args, fmt.Sprintf("%s.ccnproxy.local", serverContainer.ID[:12]))
		}

		args = append(args, fmt.Sprintf("http://%s/", choconContainer.ID[:12]))
	} else {
		if useHTTPS {
			args = append(args, fmt.Sprintf("https://%s/", serverContainer.ID[:12]))
		} else {
			args = append(args, fmt.Sprintf("http://%s/", serverContainer.ID[:12]))
		}
	}

	stdout, _, err := clientContainer.Execute("/root/go/bin/hey", args...)

	if err != nil {
		return 0, 0, 0, err
	}

	records, err := csv.NewReader(bytes.NewReader(stdout)).ReadAll()

	if err != nil {
		return 0, 0, 0, err
	}

	// The number of responses with 200 status code.
	success := 0
	// The number of responses with non-200 status code.
	fail := 0

	responseTimeSum := float64(0)

	keys := records[0]

	for _, record := range records[1:] {
		for i, key := range keys {
			if key == "status-code" {
				if record[i] == "200" {
					success++
				} else {
					fail++
				}
			} else if key == "response-time" {
				r, err := strconv.ParseFloat(record[i], 64)

				if err != nil {
					return 0, 0, 0, err
				}

				responseTimeSum += r
			}
		}
	}

	responseTime := responseTimeSum / float64(success+fail)

	return success, fail, responseTime, nil
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
		"success", "fail", "responseTime",
	)

	// When the program has received SIGTERM, 'exit' is set to true.
	exit := false
	sigCh := make(chan os.Signal)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		exit = true
	}()

	for _, bodySize := range c.BodySize.slice(100) {
		for _, latency := range c.Latency.slice(100) {
			for _, duration := range c.Duration.slice(30) {
				for memoryLimitIdx, memoryLimit := range c.MemoryLimit.slice(2048) {
					for cpuLimitIdx, cpuLimit := range c.CPULimit.slice(1) {
						for _, useHTTPS := range c.UseHTTPS.slice(true) {
							for _, useChocon := range c.UseChocon.slice(true) {
								for _, concurrency := range c.Concurrency.slice(10) {
									for _, keepAlive := range c.KeepAlive.slice(true) {
										if exit {
											return
										}

										// When chocon is not used, memoryLimit and cpuLimit have no effects.
										if !useChocon && (memoryLimitIdx != 0 || cpuLimitIdx != 0) {
											continue
										}

										success, fail, responseTime, err := run(
											useHTTPS, int(bodySize+eps), int(latency+eps), useChocon, int(duration+eps),
											int(concurrency+eps), keepAlive, 0, float32(cpuLimit), uint(memoryLimit+eps),
										)

										if err != nil {
											log.Fatal(err)
										}

										fmt.Println(
											useHTTPS, int(bodySize+eps), int(latency+eps), useChocon, int(duration+eps),
											int(concurrency+eps), keepAlive, float32(cpuLimit), uint(memoryLimit+eps),
											success, fail, responseTime,
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
