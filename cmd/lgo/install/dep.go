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

var pkgInfoCache = make(map[string]*packageInfo)

func getPackagInfo(args ...string) (infos []*packageInfo, err error) {
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
	defer func() {
		if werr := cmd.Wait(); werr != nil && err == nil {
			err = fmt.Errorf("Failed to wait go list: %v", werr)
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
		pkgInfoCache[info.ImportPath] = &info
	}
	return infos, nil
}

type packageBlackList struct {
	pkgs     map[string]bool
	prefixes []string
}

func newPackageBlackList(flag string) *packageBlackList {
	const wildcardSuffix = "/..."
	pkgs := make(map[string]bool)
	var prefixes []string
	for _, e := range strings.Split(flag, ",") {
		if strings.HasSuffix(e, wildcardSuffix) {
			e = e[:len(e)-len(wildcardSuffix)]
			prefixes = append(prefixes, e+"/")
		}
		pkgs[e] = true
	}
	return &packageBlackList{pkgs, prefixes}
}

func (l *packageBlackList) listed(path string) bool {
	if l == nil {
		return false
	}
	if l.pkgs[path] {
		return true
	}
	for _, pre := range l.prefixes {
		if strings.HasPrefix(path, pre) {
			return true
		}
	}
	return false
}

type packageBuildWalker struct {
	build func(info *packageInfo)
	log   func(msg string)

	blacklist *packageBlackList
	visited   map[string]bool
}

func (c *packageBuildWalker) walk(path string) error {
	if c.visited[path] {
		return nil
	}
	if c.visited == nil {
		c.visited = make(map[string]bool)
	}
	c.visited[path] = true

	info := pkgInfoCache[path]
	if info == nil {
		// If info is not in pkgInfoCache, get the info with go list again.
		// This logics is important to handle "testdata" and "vendor" (since 1.9.2) dir
		// properly because "..." wildcard does not match to these dirs.
		// Special handling for "testdata" exists because "..."" skips testdata.
		//
		// https://golang.org/cmd/go/#hdr-Description_of_package_lists
		// https://github.com/golang/go/issues/19090#issuecomment-290163419
		if _, err := getPackagInfo(path); err != nil {
			return err
		}
		info = pkgInfoCache[path]
		if info == nil {
			return fmt.Errorf("No package info for %s", path)
		}
	}
	if c.shouldSkipPkg(info) {
		return nil
	}
	for _, im := range info.Imports {
		if im != "C" {
			if err := c.walk(im); err != nil {
				return err
			}
		}
	}
	c.build(info)
	return nil
}

func (c *packageBuildWalker) shouldSkipPkg(info *packageInfo) bool {
	if info.Standard || info.Name == "main" || len(info.GoFiles) == 0 {
		// Ignore...
		// 1. standard libraries.
		// 2. binaries
		return true
	}
	if len(info.GoFiles) == 0 {
		// Ignore...
		// 3. test only packages like golang.org/x/debug/tests/peek.
		c.log(fmt.Sprintf("%s is ignored because it is test-only package", info.ImportPath))
		return true
	}
	if info.Stale {
		// Ignore...
		// 4. stale packages
		c.log(fmt.Sprintf("%s is ignored because it is stale: %s", info.ImportPath, info.StaleReason))
		return true
	}
	if c.blacklist.listed(info.ImportPath) {
		// 5. Blacklisted
		c.log(fmt.Sprintf("%q is blacklisted. Ignoring.", info.ImportPath))
		return true
	}

	if knownIncompatiblePkgs[info.ImportPath] || strings.HasPrefix(info.ImportPath, "github.com/yunabe/lgo/") {
		return true
	}
	return false
}

func buildThirPartyPackages(pkgDir string, blacklist *packageBlackList) {
	infos, err := getPackagInfo("...")
	if err != nil {
		log.Fatalf("Failed to list packages: %v", err)
	}
	total := 0
	countWalker := packageBuildWalker{
		build: func(*packageInfo) {
			total++
		},
		log: func(msg string) {
			log.Println(msg)
		},
		blacklist: blacklist,
	}
	for _, info := range infos {
		if err := countWalker.walk(info.ImportPath); err != nil {
			log.Fatalf("Failed to walk build deps of %q: %v", info.ImportPath, err)
		}
	}
	count := 0
	failed := make(map[string]bool)
	buildWalker := packageBuildWalker{
		build: func(info *packageInfo) {
			path := info.ImportPath
			count++
			for _, im := range info.Imports {
				if failed[im] {
					failed[path] = true
					log.Printf("(%d/%d) Skipping %q due to failures in dependencies", count, total, path)
					return
				}
			}
			log.Printf("(%d/%d) Building %q", count, total, path)
			cmd := exec.Command("go", "install", "-buildmode=shared", "-linkshared", "-pkgdir", pkgDir, path)
			cmd.Stderr = os.Stderr
			cmd.Stdout = os.Stdout
			err = cmd.Run()
			if err != nil {
				log.Printf("Failed to build the shared object of %q: %v", path, err)
				failed[path] = true
			}
		},
		log:       func(msg string) {},
		blacklist: blacklist,
	}
	for _, info := range infos {
		if err := buildWalker.walk(info.ImportPath); err != nil {
			log.Fatalf("Failed to walk build deps of %q: %v", info.ImportPath, err)
		}
	}
	if len(failed) > 0 {
		log.Fatal("Failed to build one or more third-party packages. " +
			"Please check errors in install.log in $LGOPATH and fix problems or blacklist the packages.")
	}
}
