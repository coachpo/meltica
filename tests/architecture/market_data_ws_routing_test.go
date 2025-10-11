package architecture

import (
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"
)

func TestMarketDataDependsOnSharedWSRouting(t *testing.T) {
	cfg := &packages.Config{
		Mode:  packages.NeedImports | packages.NeedName,
		Tests: false,
	}

	pkgs, err := packages.Load(cfg, "github.com/coachpo/meltica/market_data/...")
	if err != nil {
		t.Fatalf("load market_data packages: %v", err)
	}

	const publicAPI = "github.com/coachpo/meltica/lib/ws-routing"
	var offenders []string
	for _, pkg := range pkgs {
		if pkg == nil {
			continue
		}
		for importPath := range pkg.Imports {
			if strings.HasPrefix(importPath, publicAPI+"/") {
				offenders = append(offenders, pkg.PkgPath)
			}
		}
	}

	if len(offenders) > 0 {
		t.Fatalf("market_data packages depend on internal ws-routing subpackages: %v", offenders)
	}
}
