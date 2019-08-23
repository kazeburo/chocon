package docker

import (
	"bytes"
	"errors"
	"os/exec"
)

// Compose represents a set of containers.
type Compose struct {
	Containers []*Container
}

// New sets up a Compose struct. The containers names of which
// are specified here will start running when `Compose.Up` is called.
func New(containers ...string) *Compose {
	c := make([]*Container, 0, len(containers))
	for _, i := range containers {
		c = append(c, &Container{i})
	}
	return &Compose{c}
}

// Container fetches the container with the specified name.
func (c *Compose) Container(name string) (*Container, error) {
	// This is slow but admissible as the number of containers will not be that many.
	for _, c := range c.Containers {
		if c.name == name {
			return c, nil
		}
	}

	return &Container{}, errors.New("container not found")
}

// Up gets the containers running. It calls `docker-compose up` internally.
func (c *Compose) Up(detached bool) error {
	cmd := "docker-compose"
	args := []string{"up"}
	if detached {
		args = append(args, "-d")
	}
	for _, container := range c.Containers {
		args = append(args, container.name)
	}
	return exec.Command(cmd, args...).Run()
}

// Down shuts down the set of containers. It calls `docker-compose down` internally.
func (c *Compose) Down() error {
	return exec.Command("docker-compose", "down").Run()
}

// Container represends a docker container.
type Container struct {
	name string
}

// Execute a command inside the docker container. It calls `docker <container> exec ...` internally.
func (c *Container) Execute(command string, arg ...string) ([]byte, []byte, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.Command("docker", append([]string{"exec", c.name, command}, arg...)...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.Bytes(), stderr.Bytes(), err
}
