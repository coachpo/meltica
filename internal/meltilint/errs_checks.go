package meltilint

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/packages"
)

func checkErrsPackage(pkg *packages.Package) []Issue {
	var issues []Issue
	typeSpecs := collectTypeSpecs(pkg)
	if _, ok := typeSpecs["E"]; !ok {
		issues = append(issues, Issue{
			Position: pkg.Fset.Position(pkg.Syntax[0].Pos()),
			Package:  pkg.Name,
			Message:  "STD-07 errs.E type must be declared",
		})
	}
	requiredCodes := []string{"CodeRateLimited", "CodeAuth", "CodeInvalid", "CodeExchange", "CodeNetwork"}
	foundCodes := map[string]bool{}
	for _, file := range pkg.Syntax {
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.CONST {
				continue
			}
			for _, spec := range gen.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				typeName := exprString(pkg.Fset, vs.Type)
				if typeName != "Code" {
					continue
				}
				for _, name := range vs.Names {
					foundCodes[name.Name] = true
				}
			}
		}
	}
	for _, code := range requiredCodes {
		if !foundCodes[code] {
			issues = append(issues, Issue{
				Position: pkg.Fset.Position(pkg.Syntax[0].Pos()),
				Package:  pkg.Name,
				Message:  fmt.Sprintf("STD-07 errs.%s constant missing", code),
			})
		}
	}
	if fn := pkg.Types.Scope().Lookup("NotSupported"); fn != nil {
		if sig, ok := fn.Type().(*types.Signature); ok {
			if sig.Results().Len() != 1 || !isPointerTo(sig.Results().At(0).Type(), "github.com/coachpo/meltica/errs", "E") {
				issues = append(issues, Issue{
					Position: pkg.Fset.Position(fn.Pos()),
					Package:  pkg.Name,
					Message:  "STD-07 errs.NotSupported must return *errs.E",
				})
			}
		}
	}
	return issues
}

func isPointerTo(t types.Type, pkgPath, typeName string) bool {
	ptr, ok := t.(*types.Pointer)
	if !ok {
		return false
	}
	named, ok := ptr.Elem().(*types.Named)
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
