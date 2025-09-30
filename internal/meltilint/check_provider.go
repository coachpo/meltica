package meltilint

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/token"
	"strings"

	"golang.org/x/tools/go/packages"
)

type providerChecks struct {
	capabilities       map[string]token.Pos
	spotUnsupported    bool
	linearUnsupported  bool
	inverseUnsupported bool
	wsUnsupported      bool
	versionPos         token.Pos
	versionMatches     bool
}

func checkProviderPackage(pkg *packages.Package) []Issue {
	if !strings.HasPrefix(pkg.PkgPath, "github.com/coachpo/meltica/providers/") {
		return nil
	}
	checks := collectProviderInfo(pkg)
	var issues []Issue
	issues = append(issues, checkCapabilitiesAlignment(pkg, checks)...)
	issues = append(issues, checkSupportedProtocolVersion(pkg, checks)...)
	issues = append(issues, checkNoFloatUsage(pkg, "STD-26")...)
	return issues
}

func collectProviderInfo(pkg *packages.Package) providerChecks {
	result := providerChecks{capabilities: make(map[string]token.Pos)}
	for _, file := range pkg.Syntax {
		ast.Inspect(file, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.GenDecl:
				if node.Tok != token.VAR {
					return true
				}
				for _, spec := range node.Specs {
					vs, ok := spec.(*ast.ValueSpec)
					if !ok || len(vs.Names) != 1 || vs.Names[0].Name != "capabilities" || len(vs.Values) != 1 {
						continue
					}
					call, ok := vs.Values[0].(*ast.CallExpr)
					if !ok {
						continue
					}
					sel, ok := call.Fun.(*ast.SelectorExpr)
					if !ok {
						continue
					}
					if ident, ok := sel.X.(*ast.Ident); !ok || ident.Name != "core" || sel.Sel.Name != "Capabilities" {
						continue
					}
					for _, arg := range call.Args {
						capName := exprString(pkg.Fset, arg)
						if capName != "" {
							result.capabilities[capName] = arg.Pos()
						}
					}
				}
			case *ast.FuncDecl:
				if node.Recv == nil || len(node.Recv.List) == 0 {
					return true
				}
				recv := node.Recv.List[0]
				star, ok := recv.Type.(*ast.StarExpr)
				if !ok {
					return true
				}
				ident, ok := star.X.(*ast.Ident)
				if !ok || ident.Name != "Provider" {
					return true
				}
				ret := firstReturnExpr(node.Body)
				switch node.Name.Name {
				case "Spot":
					result.spotUnsupported = isUnsupportedExpr(ret)
				case "LinearFutures":
					result.linearUnsupported = isUnsupportedExpr(ret)
				case "InverseFutures":
					result.inverseUnsupported = isUnsupportedExpr(ret)
				case "WS":
					result.wsUnsupported = isUnsupportedExpr(ret)
				case "SupportedProtocolVersion":
					result.versionPos = node.Pos()
					result.versionMatches = isProtocolVersionReturn(ret)
				}
			}
			return true
		})
	}
	return result
}

func firstReturnExpr(body *ast.BlockStmt) ast.Expr {
	if body == nil {
		return nil
	}
	for _, stmt := range body.List {
		ret, ok := stmt.(*ast.ReturnStmt)
		if !ok || len(ret.Results) == 0 {
			continue
		}
		return ret.Results[0]
	}
	return nil
}

func isUnsupportedExpr(expr ast.Expr) bool {
	switch e := expr.(type) {
	case *ast.CompositeLit:
		switch t := e.Type.(type) {
		case *ast.Ident:
			return strings.HasPrefix(t.Name, "unsupported")
		case *ast.SelectorExpr:
			return strings.HasPrefix(t.Sel.Name, "unsupported")
		}
	case *ast.Ident:
		return strings.HasPrefix(e.Name, "unsupported")
	}
	return false
}

func isProtocolVersionReturn(expr ast.Expr) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	ident, ok := sel.X.(*ast.Ident)
	return ok && ident.Name == "protocol" && sel.Sel.Name == "ProtocolVersion"
}

func checkCapabilitiesAlignment(pkg *packages.Package, info providerChecks) []Issue {
	var issues []Issue
	fset := pkg.Fset
	var fallbackPos token.Pos
	if len(pkg.Syntax) > 0 {
		fallbackPos = pkg.Syntax[0].Pos()
	}
	add := func(msg string, pos token.Pos) {
		if pos == token.NoPos {
			pos = fallbackPos
		}
		issues = append(issues, Issue{
			Position: fset.Position(pos),
			Package:  pkg.Name,
			Message:  "STD-24 " + msg,
		})
	}
	checkCap := func(capName string, supported bool) {
		pos, claimed := info.capabilities[capName]
		if supported {
			if !claimed {
				add("capability "+capName+" missing for implemented API", fallbackPos)
			}
		} else if claimed {
			add("capability "+capName+" declared but corresponding API is unsupported", pos)
		}
	}
	checkCap("core.CapabilitySpotPublicREST", !info.spotUnsupported)
	checkCap("core.CapabilityLinearPublicREST", !info.linearUnsupported)
	checkCap("core.CapabilityInversePublicREST", !info.inverseUnsupported)
	checkCap("core.CapabilityWebsocketPublic", !info.wsUnsupported)
	return issues
}

func checkSupportedProtocolVersion(pkg *packages.Package, info providerChecks) []Issue {
	if info.versionMatches {
		return nil
	}
	pos := pkg.Fset.Position(info.versionPos)
	if info.versionPos == token.NoPos && len(pkg.Syntax) > 0 {
		pos = pkg.Fset.Position(pkg.Syntax[0].Pos())
	}
	return []Issue{{
		Position: pos,
		Package:  pkg.Name,
		Message:  "STD-38 SupportedProtocolVersion must return protocol.ProtocolVersion",
	}}
}

func exprString(fset *token.FileSet, expr ast.Expr) string {
	if expr == nil {
		return ""
	}
	var buf bytes.Buffer
	if err := format.Node(&buf, fset, expr); err != nil {
		return ""
	}
	return strings.TrimSpace(buf.String())
}
