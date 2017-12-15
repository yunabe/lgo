package runner

import (
	"io/ioutil"
	"os"
	"path"
	"strings"
)

func cleanSharedLibs(lgopath string, sessID *SessionID) error {
	pkg := path.Join(lgopath, "pkg")
	files, err := ioutil.ReadDir(pkg)
	if err != nil {
		return err
	}
	prefix := "libgithub.com-yunabe-lgo-" + sessID.Marshal()
	for _, f := range files {
		if strings.HasPrefix(f.Name(), prefix) && strings.HasSuffix(f.Name(), ".so") {
			if rerr := os.Remove(path.Join(pkg, f.Name())); rerr != nil && err == nil {
				err = rerr
			}
		}
	}
	if rerr := os.RemoveAll(path.Join(pkg, "github.com/yunabe/lgo", sessID.Marshal())); rerr != nil {
		err = rerr
	}
	return err
}

// CleanSession cleans up files for a session specified by sessNum.
// CleanSession returns nil when no target file exists.
func CleanSession(gopath, lgopath string, sessID *SessionID) error {
	srcErr := os.RemoveAll(path.Join(gopath, "src/github.com/yunabe/lgo", sessID.Marshal()))
	soErr := cleanSharedLibs(lgopath, sessID)
	if srcErr != nil {
		return srcErr
	}
	return soErr
}
