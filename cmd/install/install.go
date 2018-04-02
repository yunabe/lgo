package install

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"strings"
)

type packageInfo struct {
	ImportPath  string
	Name        string
	Imports     []string
	GoFiles     []string
	Standard    bool
	Stale       bool
	StaleReason string
}

// IsStdPkg returns whether the package of path is in std library.
func IsStdPkg(path string) bool {
	// cf. https://golang.org/src/cmd/go/internal/load/pkg.go
	i := strings.Index(path, "/")
	if i < 0 {
		i = len(path)
	}
	elem := path[:i]
	return !strings.Contains(elem, ".")
}

func soFileName(path string) string {
	return "lib" + strings.Replace(path, "/", "-", -1) + ".so"
}

func pkgDir(lgopath string) string {
	return path.Join(lgopath, "pkg")
}

func IsSOInstalled(lgopath string, pkg string) bool {
	_, err := os.Stat(path.Join(pkgDir(lgopath), soFileName(pkg)))
	os.IsNotExist(err)
	return err == nil
}

type SOInstaller struct {
	cache  map[string]*packageInfo
	pkgDir string
}

func NewSOInstaller(lgopath string) *SOInstaller {
	return &SOInstaller{
		cache:  make(map[string]*packageInfo),
		pkgDir: pkgDir(lgopath),
	}
}

// Install
func (si *SOInstaller) Install(patterns ...string) error {
	pkgs, err := si.GetPackageList(patterns...)
	if err != nil {
		return err
	}
	dry := walker{si: si, dryRun: true}
	for _, pkg := range pkgs {
		dry.walk(pkg.ImportPath)
	}
	w := walker{
		si:          si,
		progressAll: len(dry.visited),
		logger: func(s string) {
			fmt.Fprintln(os.Stderr, s)
		},
		stdout: os.Stdout,
		stderr: os.Stderr,
	}
	ok := true
	for _, pkg := range pkgs {
		if cok := w.walk(pkg.ImportPath); !cok {
			ok = false
		}
	}
	if !ok {
		return errors.New("failed to install .so files")
	}
	return nil
}

// GetPackage returns the packageInfo for the path.
func (si *SOInstaller) GetPackage(path string) (*packageInfo, error) {
	if pkg, ok := si.cache[path]; ok {
		return pkg, nil
	}
	pkgs, err := si.GetPackageList(path)
	if err != nil {
		return nil, err
	}
	if len(pkgs) == 0 {
		panic("pkgs must not be empty")
	}
	return pkgs[0], nil
}

func (si *SOInstaller) GetPackageList(args ...string) (infos []*packageInfo, err error) {
	cmd := exec.Command("go", append([]string{"list", "-json"}, args...)...)
	cmd.Stderr = os.Stderr
	// We do not need to close r (https://golang.org/pkg/os/exec/#Cmd.StdoutPipe).
	r, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to pipe stdout: %v", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to run go list: %v", err)
	}
	defer func() {
		if werr := cmd.Wait(); werr != nil && err == nil {
			err = fmt.Errorf("failed to wait go list: %v", werr)
		}
	}()
	br := bufio.NewReader(r)
	dec := json.NewDecoder(br)
	for {
		var info packageInfo
		err := dec.Decode(&info)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		infos = append(infos, &info)
		si.cache[info.ImportPath] = &info
	}
	return infos, nil
}

type walker struct {
	si      *SOInstaller
	visited map[string]bool

	progressAll    int
	progress       int
	dryRun         bool
	logger         func(string)
	stdout, stderr io.Writer
}

func (w *walker) logf(format string, a ...interface{}) {
	if w.logger == nil {
		return
	}
	pre := ""
	if w.progressAll > 0 {
		pre = fmt.Sprintf("(%d/%d) ", w.progress, w.progressAll)
	}
	w.logger(pre + fmt.Sprintf(format, a...))
}

func (w *walker) walk(path string) bool {
	if path == "C" || IsStdPkg(path) {
		// Ignore "C" import and std packages.
		return true
	}
	if w.visited[path] {
		return true
	}
	if w.visited == nil {
		w.visited = make(map[string]bool)
	}
	w.visited[path] = true
	pkg, err := w.si.GetPackage(path)
	if err != nil {
		w.logf("failed to get package info for %q: %v", path, err)
		w.progress++
		return false
	}
	depok := true
	for _, im := range pkg.Imports {
		if ok := w.walk(im); !ok {
			depok = false
		}
	}
	w.progress++
	if !depok {
		w.logf("skipped %q due to failures in dependencies", path)
		return false
	}
	if w.dryRun {
		return true
	}
	w.logf("installing %q", path)
	cmd := exec.Command("go", "install", "-buildmode=shared", "-linkshared", "-pkgdir", w.si.pkgDir, path)
	cmd.Stdout = w.stdout
	cmd.Stderr = w.stderr
	if err := cmd.Run(); err != nil {
		w.logf("failed to install %q: %v", path, err)
		return false
	}
	return true
}
