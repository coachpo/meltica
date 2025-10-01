package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

type templateSpec struct {
	Name string
	Body string
}

var templates = []templateSpec{
	{
		Name: "{{.Package}}.go",
		Body: `package {{.Package}}

import (
	"context"
	"time"

	"github.com/coachpo/meltica/core"
	"github.com/coachpo/meltica/protocol"
)

type Provider struct{}

func New() *Provider { return &Provider{} }

func (p *Provider) Name() string { return "{{.Provider}}" }

func (p *Provider) Capabilities() core.ProviderCapabilities { return 0 }

func (p *Provider) SupportedProtocolVersion() string { return protocol.ProtocolVersion }

func (p *Provider) Spot(ctx context.Context) core.SpotAPI { return unsupportedSpot{} }

func (p *Provider) LinearFutures(ctx context.Context) core.FuturesAPI { return unsupportedFutures{} }

func (p *Provider) InverseFutures(ctx context.Context) core.FuturesAPI { return unsupportedFutures{} }

func (p *Provider) WS() core.WS { return unsupportedWS{} }

func (p *Provider) Close() error { return nil }

type unsupportedSpot struct{}

func (unsupportedSpot) ServerTime(context.Context) (time.Time, error) { return time.Time{}, core.ErrNotSupported }

func (unsupportedSpot) Instruments(context.Context) ([]core.Instrument, error) { return nil, core.ErrNotSupported }

func (unsupportedSpot) Ticker(context.Context, string) (core.Ticker, error) { return core.Ticker{}, core.ErrNotSupported }

func (unsupportedSpot) Balances(context.Context) ([]core.Balance, error) { return nil, core.ErrNotSupported }

func (unsupportedSpot) Trades(context.Context, string, int64) ([]core.Trade, error) { return nil, core.ErrNotSupported }

func (unsupportedSpot) PlaceOrder(context.Context, core.OrderRequest) (core.Order, error) { return core.Order{}, core.ErrNotSupported }

func (unsupportedSpot) GetOrder(context.Context, string, string, string) (core.Order, error) { return core.Order{}, core.ErrNotSupported }

func (unsupportedSpot) CancelOrder(context.Context, string, string, string) error { return core.ErrNotSupported }

type unsupportedFutures struct{}

func (unsupportedFutures) Instruments(context.Context) ([]core.Instrument, error) { return nil, core.ErrNotSupported }

func (unsupportedFutures) Ticker(context.Context, string) (core.Ticker, error) { return core.Ticker{}, core.ErrNotSupported }

func (unsupportedFutures) PlaceOrder(context.Context, core.OrderRequest) (core.Order, error) {
	return core.Order{}, core.ErrNotSupported
}

func (unsupportedFutures) Positions(context.Context, ...string) ([]core.Position, error) {
	return nil, core.ErrNotSupported
}

type unsupportedWS struct{}

func (unsupportedWS) SubscribePublic(context.Context, ...string) (core.Subscription, error) {
	return nil, core.ErrNotSupported
}

func (unsupportedWS) SubscribePrivate(context.Context, ...string) (core.Subscription, error) {
	return nil, core.ErrNotSupported
}
`,
	},
	{
		Name: "sign.go",
		Body: `package {{.Package}}

// TODO: implement request signing helpers.
`,
	},
	{
		Name: "errors.go",
		Body: `package {{.Package}}

// TODO: map provider-specific errors to *errs.E.
`,
	},
	{
		Name: "status.go",
		Body: `package {{.Package}}

// TODO: translate provider order statuses to core.OrderStatus.
`,
	},
	{
		Name: "ws.go",
		Body: `package {{.Package}}

// TODO: implement public websocket subscription and decoding.
`,
	},
	{
		Name: "ws_private.go",
		Body: `package {{.Package}}

// TODO: implement private websocket subscription and decoding.
`,
	},
	{
		Name: "conformance_test.go",
		Body: `package {{.Package}}_test

import (
	"context"
	"testing"

	"github.com/coachpo/meltica/conformance"
	"github.com/coachpo/meltica/core"
	"github.com/coachpo/meltica/providers/{{.Package}}"
)

func TestConformance(t *testing.T) {
	provider := func(context.Context) (core.Provider, error) {
		return {{.Package}}.New(), nil
	}
	if err := conformance.RunAll(context.Background(), []conformance.ProviderCase{{
		Name:         "{{.Provider}}",
		Capabilities: nil,
		Factory:      provider,
	}}); err != nil {
		t.Fatalf("conformance failed: %v", err)
	}
}
`,
	},
	{
		Name: "golden_test.go",
		Body: `package {{.Package}}_test

// TODO: add golden vector tests.
`,
	},
	{
		Name: "README.md",
		Body: `# {{.Provider | title}} Provider

TODO: document capabilities, authentication, and testing instructions.
`,
	},
}

type templateData struct {
	Package  string
	Provider string
}

func main() {
	name := flag.String("name", "", "provider short name (e.g. binance)")
	outDir := flag.String("out", "", "optional output directory (defaults to providers/<name>)")
	flag.Parse()

	if strings.TrimSpace(*name) == "" {
		fatal(errors.New("--name is required"))
	}
	short := strings.ToLower(*name)
	if strings.ContainsAny(short, " /\\") {
		fatal(fmt.Errorf("invalid provider name %q", short))
	}

	root, err := findRepoRoot()
	if err != nil {
		fatal(err)
	}

	data := templateData{
		Package:  sanitizePackage(short),
		Provider: short,
	}

	baseDir := *outDir
	if baseDir == "" {
		baseDir = filepath.Join(root, "providers", data.Package)
	} else if !filepath.IsAbs(baseDir) {
		baseDir = filepath.Join(root, baseDir)
	}

	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		fatal(err)
	}

	for _, spec := range templates {
		filename, err := render(spec.Name, data)
		if err != nil {
			fatal(err)
		}
		contents, err := render(spec.Body, data)
		if err != nil {
			fatal(err)
		}
		path := filepath.Join(baseDir, filename)
		if err := writeFile(path, contents); err != nil {
			fatal(err)
		}
	}
}

func findRepoRoot() (string, error) {
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
			return "", errors.New("go.mod not found")
		}
		dir = next
	}
}

func render(tmpl string, data templateData) (string, error) {
	funcMap := template.FuncMap{
		"title": func(s string) string {
			return cases.Title(language.English).String(s)
		},
	}
	t, err := template.New("tpl").Funcs(funcMap).Parse(tmpl)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func writeFile(path, contents string) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("%s already exists", path)
	}
	return os.WriteFile(path, []byte(contents), 0o644)
}

func sanitizePackage(name string) string {
	return strings.ReplaceAll(strings.ToLower(name), "-", "")
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "barista:", err)
	os.Exit(1)
}
