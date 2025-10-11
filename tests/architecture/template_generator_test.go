package architecture

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/coachpo/meltica/internal/linter"
	exch "github.com/coachpo/meltica/internal/templates/exchange"
)

func TestExchangeSkeletonGeneratorCreatesExpectedFiles(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine caller information")
	}

	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "../.."))
	outputDir := filepath.Join(repoRoot, "exchanges", "_template_demo")

	if err := os.RemoveAll(outputDir); err != nil {
		t.Fatalf("cleanup existing skeleton: %v", err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(outputDir)
	})

	cfg := exch.Config{
		Name:      "template_demo",
		OutputDir: outputDir,
		Force:     true,
	}

	if err := exch.GenerateSkeleton(cfg); err != nil {
		t.Fatalf("generate skeleton: %v", err)
	}

	expectedFiles := []string{
		"infra/ws/client.go",
		"infra/rest/client.go",
		"routing/ws_router.go",
		"routing/rest_router.go",
		"bridge/session.go",
		"filter/adapter.go",
	}

	for _, rel := range expectedFiles {
		path := filepath.Join(cfg.OutputDir, rel)
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected file %s to exist: %v", path, err)
		}
	}

	diagnostics, err := linter.CheckPackages([]string{
		"github.com/coachpo/meltica/exchanges/_template_demo/...",
	})
	if err != nil {
		t.Fatalf("run analyzer: %v", err)
	}
	if len(diagnostics) > 0 {
		t.Fatalf("generated skeleton violates layer rules: %v", diagnostics)
	}
}
