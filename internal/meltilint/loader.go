package meltilint

import (
	"context"

	"golang.org/x/tools/go/packages"
)

// loadPackages loads the provided patterns using go/packages and returns the resulting package list.
func loadPackages(ctx context.Context, patterns []string) ([]*packages.Package, error) {
	cfg := &packages.Config{
		Context: ctx,
		Mode:    packages.NeedName | packages.NeedFiles | packages.NeedSyntax | packages.NeedModule,
	}
	return packages.Load(cfg, patterns...)
}
