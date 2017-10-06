package install

import (
	"errors"
	"flag"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/yunabe/lgo/core" // This import is also important to install the core package to GOPATH when lgo-install is installed.
)

// recordStderr invokes `tee` command to store logs and stderr of this command to install.log.
// `tee` process is used instead of internal pipes and goroutine so that logs are stored properly
// when this process is killed with SIGINT or os.Exit.
func recordStderr(lgopath string) error {
	r, w, err := os.Pipe()
	if err != nil {
		return err
	}
	tee := exec.Command("tee", "--ignore-interrupts", filepath.Join(lgopath, "install.log"))
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

	pkgInfos, err := getAllPackagInfo()
	if err != nil {
		log.Fatalf("Failed to get information about packages under $GOPATH: %v", err)
	}
	numPkgs := 0
	if errs := traverseDepsGraph(pkgInfos, func(info *packageInfo, _ map[string]bool) error {
		numPkgs++
		return nil
	}); len(errs) > 0 {
		errStrs := make([]string, len(errs))
		for i, err := range errs {
			errStrs[i] = err.Error()
		}
		log.Fatalf("Failed to count the nubmer of packages: %s", strings.Join(errStrs, "\n"))
	}

	progress := 0
	if errs := traverseDepsGraph(pkgInfos, func(info *packageInfo, failure map[string]bool) error {
		progress++

		path := info.ImportPath
		blist := strings.Split(*packageBlacklists, ",")
		for _, black := range blist {
			if strings.HasSuffix(black, "...") && strings.HasPrefix(path, black[:len(black)-len("...")]) || path == black {
				log.Printf("%q is blacklisted. Ignoring.", path)
				return nil
			}
		}
		cfail := false
		for _, im := range info.Imports {
			if failure[im] {
				cfail = true
				break
			}
		}
		if cfail {
			log.Printf("(%d/%d) Skipping %q due to failures in dependencies", progress, numPkgs, path)
			return errors.New("Skipped")
		}
		log.Printf("(%d/%d) Building %q", progress, numPkgs, path)
		cmd = exec.Command("go", "install", "-buildmode=shared", "-linkshared", "-pkgdir", pkgDir, path)
		cmd.Stderr = os.Stderr
		cmd.Stdout = os.Stdout
		err = cmd.Run()
		if err != nil {
			log.Printf("Failed to build the shared object of %q: %v", path, err)
		}
		return err
	}); len(errs) > 0 {
		log.Fatal("Failed to build one or more third-party packages. " +
			"Please check errors in install.log in $LGOPATH and fix problems or blacklist the packages.")
	}

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
