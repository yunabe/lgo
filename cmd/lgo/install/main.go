package install

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"

	"github.com/yunabe/lgo/core" // This import is also important to install the core package to GOPATH when lgo-install is installed.
)

// recordStderr invokes `tee` command to store logs and stderr of this command to install.log.
// `tee` process is used instead of internal pipes and goroutine so that logs are stored properly
// when this process is killed with SIGINT or os.Exit.
func recordStderr(lgopath string) error {
	logPath := filepath.Join(lgopath, "install.log")
	if err := func() error {
		f, err := os.Create(logPath)
		if err != nil {
			return err
		}
		return f.Close()
	}(); err != nil {
		return fmt.Errorf("Failed to create a log file: %v", err)
	}

	r, w, err := os.Pipe()
	if err != nil {
		return err
	}
	tee := exec.Command("tee", "-i", logPath)
	tee.Stdout = os.Stderr
	tee.Stderr = os.Stderr
	tee.Stdin = r
	os.Stderr = w
	log.SetOutput(w)
	return tee.Start()
}

func Main() {
	fSet := flag.NewFlagSet(os.Args[0]+" install", flag.ExitOnError)
	packageBlacklists := fSet.String("package_blacklist", "", "A commna separated list of package you don't need to use from lgo")
	cleanBeforeInstall := fSet.Bool("clean", false, "If true, clean existing files before install")
	// Ignore errors; CommandLine is set for ExitOnError.
	fSet.Parse(os.Args[2:])

	if runtime.GOOS != "linux" {
		log.Fatal("lgo only supports Linux")
	}
	root := os.Getenv("LGOPATH")
	if root == "" {
		log.Fatalf("LGOPATH environ variable is not set")
		return
	}
	root, err := filepath.Abs(root)
	if err != nil {
		log.Fatalf("Failed to get the abspath of LGOPATH: %v", err)
	}
	if err := os.MkdirAll(root, 0766); err != nil {
		log.Fatalf("Failed to create a directory on $LGOPATH: %v", err)
	}
	// Record logs into install.log and stderr.
	err = recordStderr(root)
	if err != nil {
		log.Fatalf("Failed to record logs to install.log in %s: %v", root, err)
	}

	log.Printf("Install lgo to %s", root)
	binDir := path.Join(root, "bin")
	pkgDir := path.Join(root, "pkg")
	if *cleanBeforeInstall {
		log.Printf("Clean %s before install", root)
		err = os.RemoveAll(binDir)
		if err != nil {
			log.Fatalf("Failed to clean bin dir: %v", err)
		}
		err = os.RemoveAll(pkgDir)
		if err != nil {
			log.Fatalf("Failed to clean pkg dir: %v", err)
		}
	}
	err = os.MkdirAll(binDir, 0766)
	if err != nil {
		log.Fatalf("Failed to create bin dir: %v", err)
	}
	err = os.MkdirAll(pkgDir, 0766)
	if err != nil {
		log.Fatalf("Failed to clean pkg dir: %v", err)
	}

	log.Print("Building libstd.so")
	cmd := exec.Command("go", "install", "-buildmode=shared", "-pkgdir", pkgDir, "std")
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	err = cmd.Run()
	if err != nil {
		log.Fatalf("Failed to build libstd.so: %v", err)
	}

	log.Print("Building lgo core package")
	cmd = exec.Command("go", "install", "-buildmode=shared", "-linkshared", "-pkgdir", pkgDir, core.SelfPkgPath)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	err = cmd.Run()
	if err != nil {
		log.Fatalf("Failed to build the shared object of the core library: %v", err)
	}

	log.Print("Building third-party packages in $GOPATH")
	buildThirPartyPackages(pkgDir, newPackageBlackList(*packageBlacklists))

	log.Print("Installing lgo-internal")
	cmd = exec.Command("go", "build", "-pkgdir", pkgDir, "-linkshared",
		"-o", path.Join(binDir, "lgo-internal"), "github.com/yunabe/lgo/cmd/lgo-internal")
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	err = cmd.Run()
	if err != nil {
		log.Fatalf("Failed to build lgo-internal: %v", err)
	}
	log.Printf("lgo was installed in %s successfully", root)
}
