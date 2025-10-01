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
	// Ensure sizes are computed for the active platform
	env = append(env, "GOOS="+runtime.GOOS)
	env = append(env, "GOARCH="+runtime.GOARCH)
	cfg := &packages.Config{
		Context: ctx,
		Mode: packages.NeedName |
			packages.NeedFiles |
			packages.NeedCompiledGoFiles |
			packages.NeedSyntax |
			packages.NeedTypes |
			packages.NeedTypesInfo |
			packages.NeedImports |
			packages.NeedDeps |
			packages.NeedModule,
		Env: env,
	}
	return packages.Load(cfg, patterns...)
}
