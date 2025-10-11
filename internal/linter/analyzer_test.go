package linter_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/coachpo/meltica/internal/linter"
)

func TestAnalyzer_AllowsValidImports(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, linter.Analyzer,
		"github.com/coachpo/meltica/exchanges/analyzertest/infra/pass",
		"github.com/coachpo/meltica/lib/ws-routing/pass",
	)
}

func TestAnalyzer_FlagsInvalidImports(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, linter.Analyzer,
		"github.com/coachpo/meltica/exchanges/analyzertest/infra/fail",
		"github.com/coachpo/meltica/lib/ws-routing/fail",
	)
}
