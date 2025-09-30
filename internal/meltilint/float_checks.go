package meltilint

import (
	"fmt"
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/packages"
)

func checkNoFloatUsage(pkg *packages.Package, stdID string) []Issue {
	if pkg.TypesInfo == nil {
		return nil
	}
	var issues []Issue
	for _, file := range pkg.Syntax {
		filename := pkg.Fset.Position(file.Pos()).Filename
		if strings.HasSuffix(filename, "_test.go") {
			continue
		}
		ast.Inspect(file, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.TypeSpec:
				st, ok := node.Type.(*ast.StructType)
				if !ok || !ast.IsExported(node.Name.Name) {
					return true
				}
				for _, field := range st.Fields.List {
					if len(field.Names) == 0 {
						continue
					}
					for _, name := range field.Names {
						if !ast.IsExported(name.Name) {
							continue
						}
						typ := pkg.TypesInfo.TypeOf(field.Type)
						if isFloatType(typ) {
							issues = append(issues, Issue{
								Position: pkg.Fset.Position(field.Pos()),
								Package:  pkg.Name,
								Message:  fmt.Sprintf("%s exported field %s.%s must not use float", stdID, node.Name.Name, name.Name),
							})
						}
					}
				}
			case *ast.FuncDecl:
				if !node.Name.IsExported() {
					return false
				}
				funcFile := pkg.Fset.Position(node.Pos()).Filename
				if strings.HasSuffix(funcFile, "_test.go") {
					return false
				}
				if node.Type.Params != nil {
					for _, field := range node.Type.Params.List {
						typ := pkg.TypesInfo.TypeOf(field.Type)
						if isFloatType(typ) {
							issues = append(issues, Issue{
								Position: pkg.Fset.Position(field.Pos()),
								Package:  pkg.Name,
								Message:  fmt.Sprintf("%s %s parameter type must not be float", stdID, node.Name.Name),
							})
						}
					}
				}
				if node.Type.Results != nil {
					for _, field := range node.Type.Results.List {
						typ := pkg.TypesInfo.TypeOf(field.Type)
						if isFloatType(typ) {
							issues = append(issues, Issue{
								Position: pkg.Fset.Position(field.Pos()),
								Package:  pkg.Name,
								Message:  fmt.Sprintf("%s %s return type must not be float", stdID, node.Name.Name),
							})
						}
					}
				}
			}
			return true
		})
	}
	return issues
}

func isFloatType(t types.Type) bool {
	if t == nil {
		return false
	}
	if ptr, ok := t.(*types.Pointer); ok {
		return isFloatType(ptr.Elem())
	}
	if named, ok := t.(*types.Named); ok {
		return isFloatType(named.Underlying())
	}
	basic, ok := t.(*types.Basic)
	if !ok {
		return false
	}
	return basic.Kind() == types.Float32 || basic.Kind() == types.Float64
}
