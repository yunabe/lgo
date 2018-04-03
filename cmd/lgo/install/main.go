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

	"github.com/yunabe/lgo/cmd/install"
	"github.com/yunabe/lgo/core" // This import is also important to install the core package to GOPATH when lgo-install is installed.
)

// recordStderr invokes `tee` command to store logs and stderr of this command to install.log.
// `tee` process is used instead of internal pipes and goroutine so that logs are stored properly
// when this process is killed with SIGINT or os.Exit.
func recordStderr(logPath string) error {
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

func checkEnv(logFileName string) string {
	if runtime.GOOS != "linux" {
		log.Fatal("lgo only supports Linux")
	}
	root := os.Getenv("LGOPATH")
	if root == "" {
		log.Fatalf("LGOPATH environ variable is not set")
	}
	root, err := filepath.Abs(root)
	if err != nil {
		log.Fatalf("Failed to get the abspath of LGOPATH: %v", err)
	}
	if err := os.MkdirAll(root, 0766); err != nil {
		log.Fatalf("Failed to create a directory on $LGOPATH: %v", err)
	}
	// Record logs into logFileName and stderr.
	if err := recordStderr(filepath.Join(root, logFileName)); err != nil {
		log.Fatalf("Failed to record logs to install.log in %s: %v", root, err)
	}
	return root
}

func InstallMain() {
	fSet := flag.NewFlagSet(os.Args[0]+" install", flag.ExitOnError)
	cleanBeforeInstall := fSet.Bool("clean", false, "If true, clean existing files before install")
	// Ignore errors; fSet is set for ExitOnError.
	fSet.Parse(os.Args[2:])

	root := checkEnv("install.log")
	log.Printf("Install lgo to %s", root)
	binDir := path.Join(root, "bin")
	pkgDir := path.Join(root, "pkg")
	if *cleanBeforeInstall {
		log.Printf("Clean %s before install", root)
		if err := os.RemoveAll(binDir); err != nil {
			log.Fatalf("Failed to clean bin dir: %v", err)
		}
		if err := os.RemoveAll(pkgDir); err != nil {
			log.Fatalf("Failed to clean pkg dir: %v", err)
		}
	}
	if err := os.MkdirAll(binDir, 0766); err != nil {
		log.Fatalf("Failed to create bin dir: %v", err)
	}

	if err := os.MkdirAll(pkgDir, 0766); err != nil {
		log.Fatalf("Failed to clean pkg dir: %v", err)
	}

	log.Print("Building libstd.so")
	// Note: From go1.10, libstd.so should be installed with -linkshared to avoid recompiling std libraries.
	//       go1.8 and go1.9 work regardless of the existence of -linkshared here.
	cmd := exec.Command("go", "install", "-buildmode=shared", "-linkshared", "-pkgdir", pkgDir, "std")
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	if err := cmd.Run(); err != nil {
		log.Fatalf("Failed to build libstd.so: %v", err)
	}

	log.Print("Building lgo core package")
	cmd = exec.Command("go", "install", "-buildmode=shared", "-linkshared", "-pkgdir", pkgDir, core.SelfPkgPath)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	if err := cmd.Run(); err != nil {
		log.Fatalf("Failed to build the shared object of the core library: %v", err)
	}

	log.Print("Building third-party packages in $GOPATH")
	// buildThirdPartyPackages(pkgDir, newPackageBlackList(*packageBlacklists))

	log.Print("Installing lgo-internal")
	cmd = exec.Command("go", "build", "-pkgdir", pkgDir, "-linkshared",
		"-o", path.Join(binDir, "lgo-internal"), "github.com/yunabe/lgo/cmd/lgo-internal")
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout

	if err := cmd.Run(); err != nil {
		log.Fatalf("Failed to build lgo-internal: %v", err)
	}
	log.Printf("lgo was installed in %s successfully", root)
}

func InstallPkgMain() {
	fSet := flag.NewFlagSet(os.Args[0]+" pkginstall", flag.ExitOnError)
	// Ignore errors; fSet is set for ExitOnError.
	fSet.Parse(os.Args[2:])

	root := checkEnv("installpkg.log")

	var args []string
	ok := true
	for _, arg := range fSet.Args() {
		if install.IsStdPkg(arg) {
			log.Printf("Can not install std packages: %v", arg)
			ok = false
		}
		args = append(args, arg)
	}
	if !ok {
		os.Exit(1)
	}
	if err := install.NewSOInstaller(root).Install(args...); err != nil {
		log.Fatal(err)
	}
}
