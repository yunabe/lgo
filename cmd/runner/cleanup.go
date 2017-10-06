package runner

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"path/filepath"
	"strings"
	"syscall"
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

// MainWithCleanup is a main routine called from wrapper binaries (lgo, lgo-kernel).
// If killchild is passed, the child process is killed when killchild is closed.
func MainWithCleanup(lgobin string, killchild <-chan struct{}) {
	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		log.Fatal("GOPATH is not set")
	}
	lgopath := os.Getenv("LGOPATH")
	if lgopath == "" {
		log.Fatal("LGOPATH is empty")
	}
	lgopath, err := filepath.Abs(lgopath)
	if err != nil {
		log.Fatalf("Failed to get the absolute path of LGOPATH: %v", err)
	}
	root := os.Getenv("LGOPATH")
	if root == "" {
		log.Fatalf("LGOPATH environ variable is not set")
		return
	}
	// SIGINT: Ctrl-C, SIGTERM: kill, SIGQUIT: Ctrl-\, SIGTSTP: Ctrl-z
	signal.Notify(make(chan os.Signal), syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGTSTP, syscall.SIGHUP)

	sessID := NewSessionID()
	binary := path.Join(lgopath, "bin", lgobin)
	args := []string{fmt.Sprintf("--sess_id=%s", sessID.Marshal())}
	args = append(args, os.Args[1:]...)
	cmd := exec.Command(binary, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	failed := false
	if err := cmd.Start(); err != nil {
		log.Printf("Failed to start %s with %v: %v", binary, args, err)
		failed = true
	} else {
		if killchild != nil {
			// This goroutine leaks! It's okay because this function must be called only once.
			go func() {
				<-killchild
				cmd.Process.Kill()
			}()
		}
		if err := cmd.Wait(); err != nil {
			log.Printf("%s failed: %v", binary, err)
			failed = true
		}
	}
	if err := CleanSession(gopath, lgopath, sessID); err != nil {
		log.Fatalf("Clean-up failure: %v", err)
	}
	if failed {
		os.Exit(1)
	}
}
