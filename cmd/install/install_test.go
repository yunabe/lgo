package install

import (
	"reflect"
	"sort"
	"testing"
)

func TestIsStdPkg(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"os", true},
		{"os/exec", true},
		{"os/...", true},
		{"github.com/yunabe/lgo", false},
		{"github.com/yunabe/...", false},
	}
	for _, tc := range tests {
		got := IsStdPkg(tc.path)
		if got != tc.want {
			t.Errorf("IsStdPkg(%q) = %v; want %v", tc.path, got, tc.want)
		}
	}
}

func TestGetPackageStandard(t *testing.T) {
	si := NewSOInstaller("")
	pkg, err := si.getPackage("os")
	if err != nil {
		t.Fatal(err)
	}
	if pkg.ImportPath != "os" {
		t.Errorf("pkg.ImportPath = %v; want \"os\"", pkg.ImportPath)
	}
	if !pkg.Standard {
		t.Error("pkg.Standard = false; want true")
	}
}

func TestGetPackage3P(t *testing.T) {
	si := NewSOInstaller("")
	core := "github.com/yunabe/lgo/core"
	pkg, err := si.getPackage(core)
	if err != nil {
		t.Fatal(err)
	}
	if pkg.ImportPath != core {
		t.Errorf("pkg.ImportPath = %v; want %q", pkg.ImportPath, core)
	}
	if pkg.Standard {
		t.Error("pkg.Standard = true; want false")
	}
}

func TestGetPackageList(t *testing.T) {
	si := NewSOInstaller("")
	pkgs, err := si.getPackageList("os/...")
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, pkg := range pkgs {
		paths = append(paths, pkg.ImportPath)
	}
	sort.Strings(paths)
	want := []string{"os", "os/exec", "os/signal", "os/user"}
	if !reflect.DeepEqual(paths, want) {
		t.Errorf("got %v, want %v", paths, want)
	}
}
