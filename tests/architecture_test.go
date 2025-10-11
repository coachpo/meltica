package tests

import (
	"path/filepath"
	"strings"
	testing "testing"

	"golang.org/x/tools/go/packages"
)

func TestBinanceArchitectureNoCrossLayerImports(t *testing.T) {
	repoRoot := repoRootDir(t)
	checks := []struct {
		pattern string
		forbid  []string
		descr   string
	}{
		{
			pattern: "github.com/coachpo/meltica/exchanges/binance/filter/...",
			forbid: []string{
				"github.com/coachpo/meltica/exchanges/binance/routing",
				"github.com/coachpo/meltica/market_data/framework/router",
			},
			descr: "L4 filter layer must not depend on routing layers",
		},
		{
			pattern: "github.com/coachpo/meltica/exchanges/binance/routing/...",
			forbid: []string{
				"github.com/coachpo/meltica/exchanges/binance/filter",
			},
			descr: "L2 routing layer must not depend on L4 filters",
		},
	}

	for _, check := range checks {
		pkgs := loadPackages(t, repoRoot, check.pattern)
		for _, pkg := range pkgs {
			for importPath := range pkg.Imports {
				for _, forbidden := range check.forbid {
					if importStartsWith(importPath, forbidden) {
						t.Fatalf("%s: package %s imports forbidden dependency %s", check.descr, pkg.PkgPath, importPath)
					}
				}
			}
		}
	}
}

func TestBinanceArchitectureLayerAssignments(t *testing.T) {
	repoRoot := repoRootDir(t)
	type layerPrefix struct {
		value string
		exact bool
	}

	layers := []struct {
		name     string
		prefixes []layerPrefix
	}{
		{"L1/Infra", []layerPrefix{{value: "github.com/coachpo/meltica/exchanges/binance/infra"}, {value: "github.com/coachpo/meltica/exchanges/shared/infra"}}},
		{"L2/Routing", []layerPrefix{{value: "github.com/coachpo/meltica/exchanges/binance/routing"}, {value: "github.com/coachpo/meltica/exchanges/binance/internal"}, {value: "github.com/coachpo/meltica/exchanges/binance/wsrouting"}, {value: "github.com/coachpo/meltica/market_data/framework/router"}}},
		{"L3/Business", []layerPrefix{{value: "github.com/coachpo/meltica/exchanges/binance/bridge"}, {value: "github.com/coachpo/meltica/exchanges/binance", exact: true}, {value: "github.com/coachpo/meltica/exchanges/binance/plugin"}, {value: "github.com/coachpo/meltica/exchanges/binance/telemetry"}, {value: "github.com/coachpo/meltica/market_data/processors"}}},
		{"L4/Filter", []layerPrefix{{value: "github.com/coachpo/meltica/exchanges/binance/filter"}}},
	}

	matchesPrefix := func(p layerPrefix, path string) bool {
		if p.exact {
			return path == p.value
		}
		return importStartsWith(path, p.value)
	}

	layerFor := func(pkgPath string) (string, int) {
		matches := 0
		layerName := ""
		for _, layer := range layers {
			for _, prefix := range layer.prefixes {
				if matchesPrefix(prefix, pkgPath) {
					matches++
					layerName = layer.name
				}
			}
		}
		return layerName, matches
	}

	pkgs := loadPackages(t, repoRoot, "github.com/coachpo/meltica/exchanges/binance/...")
	builder := &strings.Builder{}
	builder.WriteString("Binance Layer Mapping:\n")
	for _, pkg := range pkgs {
		layerName, matches := layerFor(pkg.PkgPath)
		if matches == 0 {
			t.Fatalf("package %s is not assigned to any architecture layer", pkg.PkgPath)
		}
		if matches > 1 {
			t.Fatalf("package %s matches multiple architecture layers", pkg.PkgPath)
		}
		builder.WriteString(" - ")
		builder.WriteString(layerName)
		builder.WriteString(": ")
		builder.WriteString(pkg.PkgPath)
		builder.WriteRune('\n')
	}

	t.Log(builder.String())
}

func repoRootDir(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatalf("determine repo root: %v", err)
	}
	return root
}

func loadPackages(t *testing.T, dir, pattern string) []*packages.Package {
	t.Helper()
	cfg := &packages.Config{Mode: packages.NeedName | packages.NeedImports, Dir: dir}
	pkgs, err := packages.Load(cfg, pattern)
	if err != nil {
		t.Fatalf("load packages for %s: %v", pattern, err)
	}
	for _, pkg := range pkgs {
		if len(pkg.Errors) > 0 {
			t.Fatalf("package %s has errors: %v", pkg.PkgPath, pkg.Errors)
		}
	}
	return pkgs
}

func importStartsWith(path, prefix string) bool {
	if strings.HasSuffix(prefix, "/...") {
		prefix = strings.TrimSuffix(prefix, "/...")
	}
	return path == prefix || strings.HasPrefix(path, prefix+"/")
}
