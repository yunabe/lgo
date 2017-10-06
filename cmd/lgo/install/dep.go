package install

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
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

// TODO: File bugs to explain reasons.
var knownIncompatiblePkgs = map[string]bool{
	"golang.org/x/sys/plan9":                    true,
	"github.com/derekparker/delve/cmd/dlv/cmds": true,
}

func traverseDepsGraph(infos []*packageInfo,
	cb func(info *packageInfo, failures map[string]bool) error) []error {
	m := make(map[string]*packageInfo)
	for _, info := range infos {
		m[info.ImportPath] = info
	}
	successes := make(map[string]bool)
	failures := make(map[string]bool)

	var errs []error

	var visit func(info *packageInfo)
	visit = func(info *packageInfo) {
		if info.Standard || info.Name == "main" || len(info.GoFiles) == 0 {
			// Ignore...
			// 1. standard libraries.
			// 2. binaries
			return
		}
		if len(info.GoFiles) == 0 {
			// Ignore...
			// 3. test only packages like golang.org/x/debug/tests/peek.
			log.Printf("%s is ignored because it is test-only package", info.ImportPath)
			return
		}
		if info.Stale {
			// Ignore...
			// 4. stale packages
			log.Printf("%s is ignored because it is stale: %s", info.ImportPath, info.StaleReason)
			return
		}
		path := info.ImportPath
		if knownIncompatiblePkgs[info.ImportPath] || strings.HasPrefix(info.ImportPath, "github.com/yunabe/lgo/") {
			return
		}
		if successes[path] || failures[path] {
			// visited
			return
		}

		for _, path := range info.Imports {
			if path == "C" {
				continue
			}
			if dep, ok := m[path]; ok {
				visit(dep)
			} else if !strings.HasSuffix(path, "/testdata") {
				// Special handling for "testdata" exists because "..."" skips testdata.
				// https://golang.org/cmd/go/#hdr-Test_packages
				errs = append(errs, fmt.Errorf("Package info for %q used from %q not found", path, info.ImportPath))
			}
		}
		if err := cb(info, failures); err == nil {
			successes[path] = true
		} else {
			errs = append(errs, err)
			failures[path] = true
		}
	}
	for _, info := range infos {
		visit(info)
	}
	return errs
}

func getAllPackagInfo() (infos []*packageInfo, err error) {
	return getPackagInfoInternal("...")
}

func getPackagInfoInternal(args ...string) (infos []*packageInfo, err error) {
	cmd := exec.Command("go", append([]string{"list", "-json"}, args...)...)
	cmd.Stderr = os.Stderr
	// We do not need to close r (https://golang.org/pkg/os/exec/#Cmd.StdoutPipe).
	r, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("Failed to pipe stdout: %v", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("Failed to run go list: %v", err)
	}
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
	}
	if err := cmd.Wait(); err != nil {
		return nil, fmt.Errorf("Failed to wait go list: %v", err)
	}
	return infos, nil
}
