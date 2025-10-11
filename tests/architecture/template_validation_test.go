package architecture

import (
	"strings"
	"testing"

	exch "github.com/coachpo/meltica/internal/templates/exchange"
)

func TestExchangeTemplatesContainLayerContracts(t *testing.T) {
	files := exch.TemplateFiles()

	expected := map[string][]string{
		"templates/infra/ws/client.go.tmpl":     {"layers.WSConnection"},
		"templates/infra/rest/client.go.tmpl":   {"layers.RESTConnection"},
		"templates/routing/ws_router.go.tmpl":   {"layers.WSRouting", "core.Topic"},
		"templates/routing/rest_router.go.tmpl": {"layers.RESTRouting"},
		"templates/bridge/session.go.tmpl":      {"layers.Business"},
		"templates/filter/adapter.go.tmpl":      {"layers.Filter"},
	}

	if len(files) != len(expected) {
		t.Fatalf("unexpected template count: got %d want %d (%v)", len(files), len(expected), files)
	}

	for path, tokens := range expected {
		if !contains(files, path) {
			t.Fatalf("missing template: %s", path)
		}
		content, err := exch.TemplateContent(path)
		if err != nil {
			t.Fatalf("read template %s: %v", path, err)
		}
		text := string(content)
		for _, token := range tokens {
			if !strings.Contains(text, token) {
				t.Fatalf("template %s missing token %q", path, token)
			}
		}
	}
}

func contains(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}
