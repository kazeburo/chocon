package pidfile

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
)

// WritePid : write pid to given file
func WritePid(pidfile string) error {
	dir, filename := filepath.Split(pidfile)
	tmpfile, err := ioutil.TempFile(dir, filename+".*")
	if err != nil {
		return errors.Wrap(err, "Cloud not create tempfile")
	}
	_, err = tmpfile.WriteString(fmt.Sprintf("%d", os.Getpid()))
	if err != nil {
		tmpfile.Close()
		return errors.Wrap(err, "Cloud not write pid to tempfile")
	}
	tmpfile.Close()
	err = os.Rename(tmpfile.Name(), pidfile)
	if err != nil {
		return errors.Wrap(err, "Cloud not rename pidfile")
	}
	return nil
}
