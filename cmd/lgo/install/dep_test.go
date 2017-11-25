package install

import (
	"testing"
)

func TestGetPackagInfoInternal(t *testing.T) {
	infos, err := getPackagInfo("os", "github.com/yunabe/lgo/converter")
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

func Test_packageBlackList(t *testing.T) {
	blacklist := newPackageBlackList("a,b/c/...")
	tests := []struct {
		path string
		want bool
	}{
		{"a", true},
		{"b", false},
		{"b/c", true},
		{"b/c/d", true},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if got := blacklist.listed(tt.path); got != tt.want {
				t.Errorf("packageBlackList.listed() = %v, want %v", got, tt.want)
			}
		})
	}
}
