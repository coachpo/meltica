package architecture

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestWSRoutingContractSuiteIsDiscoverable(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("unable to determine caller information")
	}

	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "../.."))
	contractDir := filepath.Join(repoRoot, "tests", "contract", "ws-routing")

	info, err := os.Stat(contractDir)
	if err != nil {
		t.Fatalf("ws-routing contract suite not found: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("expected %s to be a directory", contractDir)
	}

	if _, err := os.Stat(filepath.Join(contractDir, "testdata")); err != nil {
		t.Fatalf("ws-routing contract suite missing testdata fixtures: %v", err)
	}
}
