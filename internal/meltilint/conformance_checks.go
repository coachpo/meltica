package meltilint

import (
	"go/types"

	"golang.org/x/tools/go/packages"
)

func checkConformancePackage(pkg *packages.Package) []Issue {
	var issues []Issue
	if pkg.Types.Scope().Lookup("ProviderFactory") == nil {
		issues = append(issues, Issue{
			Position: pkg.Fset.Position(pkg.Syntax[0].Pos()),
			Package:  pkg.Name,
			Message:  "STD-19 type ProviderFactory must be declared",
		})
	}
	if pkg.Types.Scope().Lookup("Options") == nil {
		issues = append(issues, Issue{
			Position: pkg.Fset.Position(pkg.Syntax[0].Pos()),
			Package:  pkg.Name,
			Message:  "STD-19 type Options must be declared",
		})
	}
	obj := pkg.Types.Scope().Lookup("RunAll")
	if obj == nil {
		issues = append(issues, Issue{
			Position: pkg.Fset.Position(pkg.Syntax[0].Pos()),
			Package:  pkg.Name,
			Message:  "STD-19 RunAll entrypoint must be exported",
		})
		return issues
	}
	fn, ok := obj.(*types.Func)
	if !ok {
		issues = append(issues, Issue{
			Position: pkg.Fset.Position(obj.Pos()),
			Package:  pkg.Name,
			Message:  "STD-19 RunAll must be a function",
		})
		return issues
	}
	sig := fn.Type().(*types.Signature)
	params := sig.Params()
	if params.Len() != 3 {
		issues = append(issues, Issue{
			Position: pkg.Fset.Position(obj.Pos()),
			Package:  pkg.Name,
			Message:  "STD-19 RunAll signature must be RunAll(t *testing.T, factory ProviderFactory, opts Options)",
		})
		return issues
	}
	if !isPointerTo(params.At(0).Type(), "testing", "T") {
		issues = append(issues, Issue{
			Position: pkg.Fset.Position(obj.Pos()),
			Package:  pkg.Name,
			Message:  "STD-19 RunAll first parameter must be *testing.T",
		})
	}
	if !isNamedType(params.At(1).Type(), pkg.Types.Path(), "ProviderFactory") {
		issues = append(issues, Issue{
			Position: pkg.Fset.Position(obj.Pos()),
			Package:  pkg.Name,
			Message:  "STD-19 RunAll second parameter must be ProviderFactory",
		})
	}
	if !isNamedType(params.At(2).Type(), pkg.Types.Path(), "Options") {
		issues = append(issues, Issue{
			Position: pkg.Fset.Position(obj.Pos()),
			Package:  pkg.Name,
			Message:  "STD-19 RunAll third parameter must be Options",
		})
	}
	if sig.Results().Len() != 0 {
		issues = append(issues, Issue{
			Position: pkg.Fset.Position(obj.Pos()),
			Package:  pkg.Name,
			Message:  "STD-19 RunAll must not return values",
		})
	}
	return issues
}

func isNamedType(t types.Type, pkgPath, typeName string) bool {
	named, ok := t.(*types.Named)
	if !ok {
		return false
	}
	obj := named.Obj()
	if obj == nil {
		return false
	}
	pkg := obj.Pkg()
	if pkg == nil {
		return false
	}
	return pkg.Path() == pkgPath && obj.Name() == typeName
}
