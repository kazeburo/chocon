package docker

import (
	"io/ioutil"
	"os/exec"
	"testing"
)

// Check if the given docker-compose yaml file is valid or not.
// "docker-compose -f <file> config" is called internally, and
// its exit code is used to judge the validity.
// This returns error if something went wrong while executing `docker-compose ...`,
// e.g. `docker-compose` could not be found in any directory specified in $PATH.
func isValidYaml(file string) (bool, error) {
	err := exec.Command("docker-compose", "-f", file, "config").Run()

	if err != nil {
		switch err.(type) {
		case *exec.ExitError:
			return false, nil
		default:
			return false, err
		}
	}

	return true, nil
}

func TestMakeYaml(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "")

	if err != nil {
		t.Fatal(err)
	}

	// This should produce a valid yaml.
	out, err := makeYaml([]*Container{
		{
			Name: "client",
			Build: struct {
				Context    string
				Dockerfile string
			}{tmpDir, ""},
			Tty:    true,
			CapAdd: []string{"NET_ADMIN"},
		},
	})

	if err != nil {
		t.Fatal(err)
	}

	// Write the yaml to a file.

	tmpFile, err := ioutil.TempFile(tmpDir, "docker-compose.yml")

	if err != nil {
		t.Fatal(err)
	}

	if _, err := tmpFile.Write(out); err != nil {
		t.Fatal(err)
	}

	isValid, err := isValidYaml(tmpFile.Name())

	if err != nil {
		t.Fatal(err)
	}

	if !isValid {
		t.Fail()
	}
}
