package architecture

import (
	"testing"

	"github.com/coachpo/meltica/internal/linter"
)

func TestLayerBoundaries(t *testing.T) {
	packages := []string{
		"github.com/coachpo/meltica/core/...",
		"github.com/coachpo/meltica/exchanges/...",
		"github.com/coachpo/meltica/pipeline/...",
	}

	diagnostics, err := linter.CheckPackages(packages)
	if err != nil {
		t.Fatalf("analyzer execution failed: %v", err)
	}

	if len(diagnostics) > 0 {
		for _, d := range diagnostics {
			t.Errorf("layer boundary violation: %s", d)
		}
	}
}
