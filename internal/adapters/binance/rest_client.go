package binance

import (
	"context"
	json "github.com/goccy/go-json"
	"strings"
	"time"

	"github.com/coachpo/meltica/internal/schema"
	"github.com/sourcegraph/conc"
)

// SnapshotFetcher retrieves REST snapshot payloads.
type SnapshotFetcher interface {
	Fetch(ctx context.Context, endpoint string) ([]byte, error)
}

// SnapshotParser converts REST payloads into canonical events.
type SnapshotParser interface {
	ParseSnapshot(ctx context.Context, name string, body []byte, ingestTS time.Time) ([]*schema.Event, error)
}

// RESTPoller declares a REST snapshot poll configuration.
type RESTPoller struct {
	Name     string
	Endpoint string
	Interval time.Duration
	Parser   string
}

// RESTClient polls Binance REST endpoints on configured intervals.
type RESTClient struct {
	fetcher SnapshotFetcher
	parser  SnapshotParser
	clock   func() time.Time
}

// NewRESTClient constructs a REST snapshot client.
func NewRESTClient(fetcher SnapshotFetcher, parser SnapshotParser, clock func() time.Time) *RESTClient {
	if clock == nil {
		clock = time.Now
	}
	return &RESTClient{fetcher: fetcher, parser: parser, clock: clock}
}

// Poll executes the provided REST pollers concurrently and emits canonical events.
func (c *RESTClient) Poll(ctx context.Context, pollers []RESTPoller) (<-chan *schema.Event, <-chan error) {
	events := make(chan *schema.Event)
	errs := make(chan error, len(pollers))

	if len(pollers) == 0 {
		close(events)
		close(errs)
		return events, errs
	}

	var wg conc.WaitGroup

	for _, poller := range pollers {
		poller := poller
		if poller.Interval <= 0 {
			poller.Interval = time.Second
		}
		wg.Go(func() {
			ticker := time.NewTicker(poller.Interval)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					body, err := c.fetcher.Fetch(ctx, poller.Endpoint)
					if err != nil {
						select {
						case errs <- err:
						default:
						}
						return
					}
					
					// Extract symbol from endpoint URL and inject into response for parsing
					// Binance REST API doesn't include symbol in /api/v3/depth response
					body = injectSymbolFromURL(poller.Endpoint, body)
					
					ingest := c.clock().UTC()
					parsed, err := c.parser.ParseSnapshot(ctx, poller.Parser, body, ingest)
					if err != nil {
						select {
						case errs <- err:
						default:
						}
						continue
					}
					for _, evt := range parsed {
						if evt == nil {
							continue
						}
						if evt.IngestTS.IsZero() {
							evt.IngestTS = ingest
						}
						if evt.EmitTS.IsZero() {
							evt.EmitTS = ingest
						}
						select {
						case <-ctx.Done():
							return
						case events <- evt:
						}
					}
				}
			}
		})
	}

	go func() {
		defer close(events)
		defer close(errs)
		wg.Wait()
	}()

	return events, errs
}

// injectSymbolFromURL extracts symbol from URL query parameter and injects it into JSON response
// Binance /api/v3/depth doesn't include "s" (symbol) field, so we need to add it
func injectSymbolFromURL(endpoint string, body []byte) []byte {
	// Extract symbol from URL: /api/v3/depth?symbol=BTCUSDT&limit=1000
	symbol := extractSymbolFromURL(endpoint)
	if symbol == "" {
		return body
	}
	
	// Parse existing JSON
	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return body // Return original on error
	}
	
	// Add symbol field
	data["s"] = symbol
	
	// Re-encode
	modified, err := json.Marshal(data)
	if err != nil {
		return body // Return original on error
	}
	
	return modified
}

// extractSymbolFromURL extracts the symbol query parameter from a URL
func extractSymbolFromURL(endpoint string) string {
	// Look for symbol= in the URL
	idx := strings.Index(endpoint, "symbol=")
	if idx == -1 {
		return ""
	}
	
	// Extract everything after "symbol="
	rest := endpoint[idx+7:] // len("symbol=") == 7
	
	// Find end (either & or end of string)
	endIdx := strings.IndexAny(rest, "&?#")
	if endIdx == -1 {
		return rest
	}
	
	return rest[:endIdx]
}
