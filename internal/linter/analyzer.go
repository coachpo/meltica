package linter

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/packages"
)

// Analyzer enforces layer boundary constraints across the project.
var Analyzer = &analysis.Analyzer{
	Name: "layerdeps",
	Doc:  "verify that packages only import from permitted architecture layers",
	Run:  run,
}

func run(pass *analysis.Pass) (interface{}, error) {
	reporter := func(pos token.Pos, msg string) {
		pass.Reportf(pos, "%s", msg)
	}
	runChecks(pass.Pkg.Path(), pass.Files, pass.Fset, reporter)
	return nil, nil
}

// CheckPackages runs the analyzer over the provided package patterns and
// returns any diagnostics discovered.
func CheckPackages(patterns []string) ([]string, error) {
	cfg := &packages.Config{
		Mode:  packages.NeedName | packages.NeedSyntax | packages.NeedFiles | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedModule | packages.NeedImports,
		Tests: true,
	}

	pkgs, err := packages.Load(cfg, patterns...)
	if err != nil {
		return nil, err
	}

	var diagnostics []string
	for _, pkg := range pkgs {
		if pkg == nil || pkg.Fset == nil || len(pkg.Syntax) == 0 {
			continue
		}

		reporter := func(pos token.Pos, msg string) {
			position := pkg.Fset.Position(pos)
			recorded := fmt.Sprintf("%s: %s", position, msg)
			diagnostics = append(diagnostics, recorded)
		}

		runChecks(pkg.PkgPath, pkg.Syntax, pkg.Fset, reporter)
	}

	return diagnostics, nil
}

func runChecks(pkgPath string, files []*ast.File, fset *token.FileSet, report func(token.Pos, string)) {
	pkgLayer := resolveLayer(pkgPath)
	isFrameworkPkg := strings.HasPrefix(pkgPath, modulePath+"/lib/ws-routing")
	for _, file := range files {
		for _, importSpec := range file.Imports {
			if importSpec.Path == nil {
				continue
			}

			path := strings.Trim(importSpec.Path.Value, "\"")
			if isFrameworkPkg && strings.HasPrefix(path, modulePath+"/market_data") {
				report(importSpec.Path.Pos(), fmt.Sprintf("framework package %q cannot import domain package %q", pkgPath, path))
				continue
			}
			targetLayer := resolveLayer(path)
			if isAllowedDependency(pkgLayer, targetLayer) {
				continue
			}

			allowed := allowedLayerNames(pkgLayer)
			suggestion := ""
			if len(allowed) > 0 {
				suggestion = fmt.Sprintf("; allowed layers: %s", strings.Join(allowed, ", "))
			}

			message := fmt.Sprintf(
				"layer boundary violation: %q (%s) cannot import %q (%s)%s",
				pkgPath,
				pkgLayer,
				path,
				targetLayer,
				suggestion,
			)
			report(importSpec.Path.Pos(), message)
		}
	}
}
