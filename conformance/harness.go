package conformance

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/yourorg/meltica/core"
	"github.com/yourorg/meltica/protocol"
)

// ProviderFactory constructs a provider for testing.
type ProviderFactory func(ctx context.Context) (core.Provider, error)

// ProviderCase describes one provider to validate.
type ProviderCase struct {
	Name         string
	Capabilities []core.Capability
	Factory      ProviderFactory
}

// RunAll executes schema and capability validations for the supplied providers.
func RunAll(ctx context.Context, providers []ProviderCase) error {
	if err := validateSchemas(); err != nil {
		return fmt.Errorf("schema validation: %w", err)
	}
	for _, pc := range providers {
		if err := runProvider(ctx, pc); err != nil {
			return fmt.Errorf("provider %s: %w", pc.Name, err)
		}
	}
	return nil
}

func validateSchemas() error {
	root, err := projectRoot()
	if err != nil {
		return err
	}
	base := filepath.Join(root, "protocol", "schemas")
	entries, err := os.ReadDir(base)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		path := filepath.Join(base, entry.Name())
		schema, err := compileSchema(path)
		if err != nil {
			return err
		}
		if err := validateVectors(root, schema, entry.Name()); err != nil {
			return err
		}
	}
	return nil
}

// ValidateSchemas exposes schema validation for tooling and CI integration.
func ValidateSchemas() error {
	return validateSchemas()
}

func compileSchema(path string) (*jsonschema.Schema, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read schema %s: %w", path, err)
	}
	var doc any
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse schema %s: %w", path, err)
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource(path, doc); err != nil {
		return nil, fmt.Errorf("add resource %s: %w", path, err)
	}
	schema, err := compiler.Compile(path)
	if err != nil {
		return nil, fmt.Errorf("compile %s: %w", path, err)
	}
	return schema, nil
}

func validateVectors(root string, schema *jsonschema.Schema, schemaFile string) error {
	base := strings.TrimSuffix(schemaFile, filepath.Ext(schemaFile))
	pattern := filepath.Join(root, "protocol", "vectors", base+"_*.json")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}
	for _, path := range matches {
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		var payload any
		if err := json.NewDecoder(file).Decode(&payload); err != nil {
			file.Close()
			return fmt.Errorf("decode %s: %w", path, err)
		}
		file.Close()
		if err := schema.Validate(payload); err != nil {
			return fmt.Errorf("%s does not satisfy %s: %w", path, schemaFile, err)
		}
	}
	return nil
}

func projectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		next := filepath.Dir(dir)
		if next == dir {
			return "", fmt.Errorf("go.mod not found from %s", dir)
		}
		dir = next
	}
}

func runProvider(ctx context.Context, pc ProviderCase) error {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	p, err := pc.Factory(ctx)
	if err != nil {
		return fmt.Errorf("factory: %w", err)
	}
	defer p.Close()
	if v := p.SupportedProtocolVersion(); v != protocol.ProtocolVersion {
		return fmt.Errorf("protocol version mismatch: provider=%s core=%s", v, protocol.ProtocolVersion)
	}
	caps := p.Capabilities()
	for _, cap := range pc.Capabilities {
		if !caps.Has(cap) {
			return fmt.Errorf("missing capability %v", cap)
		}
	}
	return nil
}
