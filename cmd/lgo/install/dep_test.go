package install

import (
	"strings"
	"testing"
)

func TestGetPackagInfoInternal(t *testing.T) {
	infos, err := getPackagInfoInternal("os", "github.com/yunabe/lgo/converter")
	if err != nil {
		t.Error(err)
	}
	if len(infos) != 2 {
		t.Errorf("Unexpected len(infos): %v", len(infos))
	}
	osInfo := infos[0]
	if osInfo.ImportPath != "os" {
		t.Errorf("Unexpected path: %s", osInfo.ImportPath)
	}
	if !osInfo.Standard {
		t.Errorf("os package must be Standard")
	}
	if len(osInfo.Imports) == 0 {
		t.Errorf("os must import at least one package")
	}

	customInfo := infos[1]
	if customInfo.Standard {
		t.Error("github.com/yunabe/lgo/converter must not be a standard package.")
	}
}

func TestGetPackagInfoInternalVendor(t *testing.T) {
	// This test confirms "..." in go list command matches to vendor package
	// despite the rule written in
	// https: //golang.org/cmd/go/#hdr-Description_of_package_lists
	infos, err := getPackagInfoInternal("github.com/yunabe/lgo/...")
	if err != nil {
		t.Error(err)
	}
	found := false
	for _, info := range infos {
		if strings.HasPrefix(info.ImportPath, "github.com/yunabe/lgo/vendor/") {
			found = true
			break
		}
	}
	if !found {
		t.Error("No vendor package found under github.com")
	}
}
