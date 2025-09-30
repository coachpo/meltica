package meltilint

import (
	"fmt"
	"go/constant"
	"go/types"
	"regexp"

	"golang.org/x/tools/go/packages"
)

var semverPattern = regexp.MustCompile(`^\d+\.\d+(\.\d+)?(-[0-9A-Za-z.-]+)?$`)

func checkProtocolPackage(pkg *packages.Package) []Issue {
	var issues []Issue
	obj := pkg.Types.Scope().Lookup("ProtocolVersion")
	if obj == nil {
		issues = append(issues, Issue{
			Position: pkg.Fset.Position(pkg.Syntax[0].Pos()),
			Package:  pkg.Name,
			Message:  "STD-34 ProtocolVersion constant must be declared",
		})
		return issues
	}
	cnst, ok := obj.(*types.Const)
	if !ok {
		issues = append(issues, Issue{
			Position: pkg.Fset.Position(obj.Pos()),
			Package:  pkg.Name,
			Message:  "STD-34 ProtocolVersion must be a constant string",
		})
		return issues
	}
	val := cnst.Val()
	if val.Kind() != constant.String {
		issues = append(issues, Issue{
			Position: pkg.Fset.Position(obj.Pos()),
			Package:  pkg.Name,
			Message:  "STD-35 ProtocolVersion must be a string literal",
		})
		return issues
	}
	str := constant.StringVal(val)
	if !semverPattern.MatchString(str) {
		issues = append(issues, Issue{
			Position: pkg.Fset.Position(obj.Pos()),
			Package:  pkg.Name,
			Message:  fmt.Sprintf("STD-35 ProtocolVersion must be valid semver, got %q", str),
		})
	}
	return issues
}
