package meltilint

import (
	"context"
	"os"
	"runtime"

	"golang.org/x/tools/go/packages"
)

// loadPackages loads the provided patterns using go/packages and returns the resulting package list.
func loadPackages(ctx context.Context, patterns []string) ([]*packages.Package, error) {
	env := os.Environ()
	if runtime.GOARCH == "arm64" {
		env = append(env, "GOARCH=amd64")
	}
	cfg := &packages.Config{
		Context: ctx,
		Mode: packages.NeedName |
			packages.NeedFiles |
			packages.NeedSyntax |
			packages.NeedTypes |
			packages.NeedTypesInfo |
			packages.NeedTypesSizes |
			packages.NeedImports |
			packages.NeedDeps |
			packages.NeedModule,
		Env: env,
	}
	return packages.Load(cfg, patterns...)
}
