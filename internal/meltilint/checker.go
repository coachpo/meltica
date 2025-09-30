package meltilint

import (
	"context"
	"errors"
	"go/token"
	"strings"
)

// Run executes all available lint checks for the provided package patterns.
func Run(ctx context.Context, patterns []string) ([]Issue, error) {
	pkgList, err := loadPackages(ctx, patterns)
	if err != nil {
		return nil, err
	}
	var allIssues []Issue
	root, rootErr := projectRoot()
	checkedProtocolDocs := false
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
		path := pkg.PkgPath
		switch {
		case path == "github.com/coachpo/meltica/core":
			allIssues = append(allIssues, checkCorePackage(pkg)...)
		case path == "github.com/coachpo/meltica/errs":
			allIssues = append(allIssues, checkErrsPackage(pkg)...)
		case path == "github.com/coachpo/meltica/protocol":
			allIssues = append(allIssues, checkProtocolPackage(pkg)...)
		case strings.HasPrefix(path, "github.com/coachpo/meltica/conformance"):
			allIssues = append(allIssues, checkConformancePackage(pkg)...)
		case strings.HasPrefix(path, "github.com/coachpo/meltica/providers/"):
			allIssues = append(allIssues, checkProviderPackage(pkg)...)
		}
		if !checkedProtocolDocs && rootErr == nil && (path == "github.com/coachpo/meltica/protocol" || strings.HasPrefix(path, "github.com/coachpo/meltica/providers/") || path == "github.com/coachpo/meltica/core") {
			allIssues = append(allIssues, checkProtocolArtifacts(root)...)
			checkedProtocolDocs = true
		}
	}
	if rootErr == nil {
		allIssues = append(allIssues, checkDocumentationSet(root)...)
	}
	return allIssues, nil
}

// ErrFailed indicates that lint issues were detected.
var ErrFailed = errors.New("meltilint: issues found")
