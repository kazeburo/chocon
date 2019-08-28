package docker

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"

	"gopkg.in/yaml.v2"
)

// Compose represents a set of containers.
type Compose struct {
	Containers []*Container
}

// New sets up a Compose struct. The containers names of which
// are specified here will start running when `Compose.Up` is called.
func New(containers ...*Container) *Compose {
	return &Compose{containers}
}

// Container fetches the container with the specified name.
func (c *Compose) Container(name string) (*Container, error) {
	// This is slow but admissible as the number of containers will not be that many.
	for _, c := range c.Containers {
		if c.Name == name {
			return c, nil
		}
	}

	return &Container{}, errors.New("container not found")
}

// Up gets the containers running. It calls `docker-compose up` internally.
func (c *Compose) Up(detached bool, build bool) error {
	yamlOut, err := makeYaml(c.Containers)

	if err != nil {
		return err
	}

	if err := ioutil.WriteFile("./docker-compose.yml", yamlOut, 0644); err != nil {
		return err
	}

	cmd := "docker-compose"
	args := []string{"up"}

	if detached {
		args = append(args, "-d")
	}

	if build {
		args = append(args, "--build")
	}

	return exec.Command(cmd, args...).Run()
}

// Down shuts down the set of containers. It calls `docker-compose down` internally.
func (c *Compose) Down() error {
	err := exec.Command("docker-compose", "down").Run()

	if err != nil {
		return err
	}

	return os.Remove("./docker-compose.yml")
}

// Container represends a docker container.
type Container struct {
	Name  string
	Build struct {
		Context string
		// This can be omitted.
		Dockerfile string
	}
	CapAdd []string
	Tty    bool
	// Set this to 0.5 in order to limit cpu usage to 50%.
	Cpus float32
	// Set this to 1024 in order to limit memory usage to 1024MB.
	MemLimit uint
	Ports    []struct {
		Host  uint
		Guest uint
	}
	Environment []struct {
		Key   string
		Value string
	}
}

// Execute a command inside the docker container. It calls `docker <container> exec ...` internally.
func (c *Container) Execute(command string, arg ...string) ([]byte, []byte, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.Command("docker", append([]string{"exec", c.Name, command}, arg...)...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.Bytes(), stderr.Bytes(), err
}

func makeYaml(containers []*Container) ([]byte, error) {
	m := make(map[string]interface{})

	m["version"] = "2.2"

	{
		mm := make(map[string]interface{})

		for _, container := range containers {
			mmm := make(map[string]interface{})

			mmm["container_name"] = container.Name

			mmm["tty"] = container.Tty

			if container.Cpus != 0 {
				mmm["cpus"] = fmt.Sprintf("%f", container.Cpus)
			}

			if container.MemLimit != 0 {
				mmm["mem_limit"] = fmt.Sprintf("%dm", container.MemLimit)
			}

			if container.Build.Context != "" && container.Build.Dockerfile != "" {
				mmmm := make(map[string]interface{})
				mmmm["context"] = container.Build.Context
				mmmm["dockerfile"] = container.Build.Dockerfile

				mmm["build"] = mmmm
			} else if container.Build.Context != "" {
				mmm["build"] = container.Build.Context
			}

			if len(container.CapAdd) > 0 {
				mmm["cap_add"] = container.CapAdd
			}

			if len(container.Ports) > 0 {
				l := []string{}
				for _, i := range container.Ports {
					l = append(l, fmt.Sprintf("%d:%d", i.Host, i.Guest))
				}
				mmm["ports"] = l
			}

			if len(container.Environment) > 0 {
				l := []string{}
				for _, i := range container.Environment {
					l = append(l, fmt.Sprintf("%s=%s", i.Key, i.Value))
				}
				mmm["environment"] = l
			}

			mm[container.Name] = mmm
		}

		m["services"] = mm
	}

	return yaml.Marshal(m)
}
