package exchange

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"unicode"
)

// Config describes the desired exchange skeleton output.
type Config struct {
	Name      string
	OutputDir string
	Force     bool
}

var templateMappings = []struct {
	templatePath string
	outputPath   string
}{
	{"templates/infra/ws/client.go.tmpl", "infra/ws/client.go"},
	{"templates/infra/rest/client.go.tmpl", "infra/rest/client.go"},
	{"templates/routing/ws_router.go.tmpl", "routing/ws_router.go"},
	{"templates/routing/rest_router.go.tmpl", "routing/rest_router.go"},
	{"templates/bridge/session.go.tmpl", "bridge/session.go"},
	{"templates/filter/adapter.go.tmpl", "filter/adapter.go"},
}

// GenerateSkeleton renders the exchange templates into the output directory.
func GenerateSkeleton(cfg Config) error {
	if strings.TrimSpace(cfg.Name) == "" {
		return errors.New("exchange name is required")
	}
	if strings.TrimSpace(cfg.OutputDir) == "" {
		return errors.New("output directory is required")
	}

	data := map[string]string{
		"Exchange":      cfg.Name,
		"ExchangeTitle": toTitle(cfg.Name),
	}

	for _, mapping := range templateMappings {
		if err := renderTemplate(mapping.templatePath, filepath.Join(cfg.OutputDir, mapping.outputPath), data, cfg.Force); err != nil {
			return err
		}
	}
	return nil
}

func renderTemplate(templatePath, outputPath string, data any, force bool) error {
	content, err := fs.ReadFile(templateFS, templatePath)
	if err != nil {
		return fmt.Errorf("read template %s: %w", templatePath, err)
	}

	if !force {
		if _, err := os.Stat(outputPath); err == nil {
			return fmt.Errorf("output file already exists: %s", outputPath)
		}
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("create directory %s: %w", filepath.Dir(outputPath), err)
	}

	tmpl, err := template.New(filepath.Base(templatePath)).Parse(string(content))
	if err != nil {
		return fmt.Errorf("parse template %s: %w", templatePath, err)
	}

	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("create output %s: %w", outputPath, err)
	}
	defer file.Close()

	if err := tmpl.Execute(file, data); err != nil {
		return fmt.Errorf("execute template %s: %w", templatePath, err)
	}
	return nil
}

func toTitle(name string) string {
	fields := strings.FieldsFunc(name, func(r rune) bool {
		return !(unicode.IsLetter(r) || unicode.IsDigit(r))
	})
	for i, part := range fields {
		if part == "" {
			continue
		}
		runes := []rune(part)
		upper := strings.ToUpper(string(runes[0:1]))
		lower := ""
		if len(runes) > 1 {
			lower = strings.ToLower(string(runes[1:]))
		}
		fields[i] = upper + lower
	}
	return strings.Join(fields, " ")
}

// TemplateFiles lists the embedded template files.
func TemplateFiles() []string {
	files := make([]string, 0, len(templateMappings))
	for _, mapping := range templateMappings {
		files = append(files, mapping.templatePath)
	}
	return files
}

// TemplateContent returns the raw template bytes for inspection.
func TemplateContent(path string) ([]byte, error) {
	return fs.ReadFile(templateFS, path)
}
