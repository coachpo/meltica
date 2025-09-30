package protolint

import (
	"context"
	"errors"
	"go/token"
)

// Run executes all available lint checks for the provided package patterns.
func Run(ctx context.Context, patterns []string) ([]Issue, error) {
	pkgList, err := loadPackages(ctx, patterns)
	if err != nil {
		return nil, err
	}
	var allIssues []Issue
	for _, pkg := range pkgList {
		for _, pkgErr := range pkg.Errors {
			allIssues = append(allIssues, Issue{
				Position: token.Position{Filename: pkgErr.Pos},
				Package:  pkg.Name,
				Message:  pkgErr.Msg,
			})
		}
		if pkg.Types == nil || pkg.Fset == nil {
			continue
		}
		issues := checkProviderPackage(pkg)
		allIssues = append(allIssues, issues...)
	}
	return allIssues, nil
}

// ErrFailed indicates that lint issues were detected.
var ErrFailed = errors.New("protolint: issues found")
