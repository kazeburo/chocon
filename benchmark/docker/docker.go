package docker

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// Network represents a docker network.
type Network struct {
	Name string
	ID   string
}

// CreateNetwork creates a network with the given name.
// It calls `docker network create ...` internally.
func CreateNetwork(name string) (Network, error) {
	stdout := bytes.Buffer{}
	c := exec.Command("docker", "network", "create", name)
	c.Stdout = &stdout
	if err := c.Run(); err != nil {
		return Network{}, err
	}
	return Network{name, strings.TrimSpace(stdout.String())}, nil
}

// Remove removes the network.
func (n *Network) Remove() error {
	return exec.Command("docker", "network", "rm", n.ID).Run()
}

// Image represents a docker image.
type Image struct {
	ID string
}

// BuildImage builds a docker image.
// It accepts the context path and the path to a dockerfile.
// If the dockerfile is just inside the context, dockerfile can be an empty string.
func BuildImage(context, dockerfile string) (Image, error) {
	cmd := "docker"

	args := []string{"build", "-q"}
	if dockerfile != "" {
		args = append(args, "-f", dockerfile)
	}
	args = append(args, context)

	stdout := bytes.Buffer{}
	c := exec.Command(cmd, args...)
	c.Stdout = &stdout
	if err := c.Run(); err != nil {
		return Image{}, err
	}

	return Image{strings.TrimSpace(stdout.String())}, nil
}

// RunConfig is for configuring the container.
type RunConfig struct {
	Network *Network
	Tty     bool
	// 1.0 for 1 cpu
	Cpus float32
	// 1024 for 1GB
	MemLimit     uint
	Environments []struct {
		Key   string
		Value string
	}
	CapAdds []string
}

// Run runs a container of the image in detached mode.
func (image Image) Run(config *RunConfig) (Container, error) {
	cmd := "docker"

	args := []string{"run", "-d"}

	if config.Network != nil {
		args = append(args, fmt.Sprintf("--net=%s", config.Network.ID))
	}

	if config.Tty {
		args = append(args, "-t")
	}

	if config.Cpus != 0 {
		args = append(args, fmt.Sprintf("--cpus=%f", config.Cpus))
	}

	if config.MemLimit != 0 {
		args = append(args, fmt.Sprintf("--memory=%dm", config.MemLimit))
	}

	for _, environment := range config.Environments {
		args = append(args, "-e", fmt.Sprintf("%s=%s", environment.Key, environment.Value))
	}

	for _, capAdd := range config.CapAdds {
		args = append(args, fmt.Sprintf("--cap-add=%s", capAdd))
	}

	args = append(args, image.ID)

	stdout := bytes.Buffer{}
	c := exec.Command(cmd, args...)
	c.Stdout = &stdout

	if err := c.Run(); err != nil {
		return Container{}, err
	}

	return Container{strings.TrimSpace(stdout.String())}, nil
}

// Container represends a docker container.
type Container struct {
	ID string
}

// Stop stops the container.
func (c *Container) Stop() error {
	return exec.Command("docker", "stop", c.ID).Run()
}

// Execute executes a command inside the docker container.
// The contents written to stdout and stderr are returned.
// It calls `docker exec ...` internally.
func (c *Container) Execute(command string, arg ...string) ([]byte, []byte, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.Command("docker", append([]string{"exec", c.ID, command}, arg...)...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.Bytes(), stderr.Bytes(), err
}
