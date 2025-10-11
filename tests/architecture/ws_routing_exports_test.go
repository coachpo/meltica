package architecture_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

var bannedTokens = []string{"market", "trade", "ticker", "exchange", "orderbook"}

var allowedSubdirs = map[string]struct{}{
	"api":        {},
	"connection": {},
	"handler":    {},
	"internal":   {},
	"router":     {},
	"session":    {},
	"telemetry":  {},
}

func TestWSRoutingExportsAreDomainAgnostic(t *testing.T) {
	root := filepath.Join("..", "..", "lib", "ws-routing")
	violations := make([]string, 0)
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if path == root {
				return nil
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			parts := strings.Split(rel, string(filepath.Separator))
			if len(parts) > 0 {
				if _, ok := allowedSubdirs[parts[0]]; !ok {
					return fs.SkipDir
				}
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		parts := strings.Split(rel, string(filepath.Separator))
		if len(parts) == 1 {
			return nil
		}
		if len(parts) > 1 {
			if _, ok := allowedSubdirs[parts[0]]; !ok {
				return nil
			}
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			return err
		}
		checkDeclarations(file, path, &violations)
		return nil
	})
	require.NoError(t, err)
	if len(violations) > 0 {
		for _, violation := range violations {
			t.Errorf("%s", violation)
		}
		t.Fatalf("found %d domain-specific export references", len(violations))
	}
}

func checkDeclarations(file *ast.File, path string, violations *[]string) {
	addViolation := func(name, kind, detail string) {
		lower := strings.ToLower(name)
		for _, token := range bannedTokens {
			if strings.Contains(lower, token) {
				*violations = append(*violations, path+": "+kind+" `"+name+"` contains token `"+token+"`")
				return
			}
		}
	}
	checkComment := func(comment *ast.CommentGroup, subject, kind string) {
		if comment == nil {
			return
		}
		text := strings.ToLower(comment.Text())
		for _, token := range bannedTokens {
			if strings.Contains(text, token) {
				*violations = append(*violations, path+": doc for "+kind+" `"+subject+"` references token `"+token+"`")
				return
			}
		}
	}
	for _, decl := range file.Decls {
		switch typed := decl.(type) {
		case *ast.GenDecl:
			for _, spec := range typed.Specs {
				switch spec := spec.(type) {
				case *ast.TypeSpec:
					if spec.Name.IsExported() {
						addViolation(spec.Name.Name, "type", "")
						checkComment(spec.Doc, spec.Name.Name, "type")
						checkComment(typed.Doc, spec.Name.Name, "type")
					}
					switch node := spec.Type.(type) {
					case *ast.StructType:
						for _, field := range node.Fields.List {
							for _, name := range field.Names {
								if name.IsExported() {
									addViolation(name.Name, "struct field", "")
									checkComment(field.Doc, name.Name, "struct field")
								}
							}
						}
					case *ast.InterfaceType:
						for _, field := range node.Methods.List {
							for _, name := range field.Names {
								if name.IsExported() {
									addViolation(name.Name, "interface method", "")
									checkComment(field.Doc, name.Name, "interface method")
								}
							}
						}
					}
				case *ast.ValueSpec:
					for _, name := range spec.Names {
						if name.IsExported() {
							addViolation(name.Name, "value", "")
							checkComment(spec.Doc, name.Name, "value")
							checkComment(typed.Doc, name.Name, "value")
						}
					}
				}
			}
		case *ast.FuncDecl:
			if typed.Name.IsExported() {
				addViolation(typed.Name.Name, "function", "")
				checkComment(typed.Doc, typed.Name.Name, "function")
			}
		}
	}
}
