package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"text/tabwriter"

	"github.com/coachpo/meltica/core"
	exchangecap "github.com/coachpo/meltica/core/exchanges/capabilities"
	sharedplugin "github.com/coachpo/meltica/exchanges/shared/plugin"

	_ "github.com/coachpo/meltica/exchanges/binance/plugin"
)

type matrixRow struct {
	Name         string            `json:"name"`
	Protocol     string            `json:"protocol"`
	Categories   map[string]bool   `json:"categories"`
	Capabilities []string          `json:"capabilities"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

type category struct {
	key    string
	header string
	all    []core.Capability
}

var matrixCategories = []category{
	{key: "spot_trading", header: "Spot", all: toCoreCaps(exchangecap.SpotTrading)},
	{key: "linear_trading", header: "Linear", all: toCoreCaps(exchangecap.LinearTrading)},
	{key: "inverse_trading", header: "Inverse", all: toCoreCaps(exchangecap.InverseTrading)},
	{key: "market_data", header: "MarketData", all: toCoreCaps(exchangecap.CoreMarketData)},
	{key: "public_ws", header: "PublicWS", all: []core.Capability{core.CapabilityWebsocketPublic}},
	{key: "private_ws", header: "PrivateWS", all: []core.Capability{core.CapabilityWebsocketPrivate}},
	{key: "account", header: "Account", all: []core.Capability{core.CapabilityAccountBalances, core.CapabilityAccountPositions}},
	{key: "transfers", header: "Transfers", all: []core.Capability{core.CapabilityAccountTransfers}},
}

var orderedCapabilities = []core.Capability{
	core.CapabilitySpotPublicREST,
	core.CapabilitySpotTradingREST,
	core.CapabilityLinearPublicREST,
	core.CapabilityLinearTradingREST,
	core.CapabilityInversePublicREST,
	core.CapabilityInverseTradingREST,
	core.CapabilityWebsocketPublic,
	core.CapabilityWebsocketPrivate,
	core.CapabilityTradingSpotAmend,
	core.CapabilityTradingSpotCancel,
	core.CapabilityTradingLinearAmend,
	core.CapabilityTradingLinearCancel,
	core.CapabilityTradingInverseAmend,
	core.CapabilityTradingInverseCancel,
	core.CapabilityAccountBalances,
	core.CapabilityAccountPositions,
	core.CapabilityAccountMargin,
	core.CapabilityAccountTransfers,
	core.CapabilityMarketTrades,
	core.CapabilityMarketTicker,
	core.CapabilityMarketOrderBook,
	core.CapabilityMarketCandles,
	core.CapabilityMarketFundingRates,
	core.CapabilityMarketMarkPrice,
	core.CapabilityMarketLiquidations,
}

var capabilityLabels = map[core.Capability]string{
	core.CapabilitySpotPublicREST:       "spot_public_rest",
	core.CapabilitySpotTradingREST:      "spot_trading_rest",
	core.CapabilityLinearPublicREST:     "linear_public_rest",
	core.CapabilityLinearTradingREST:    "linear_trading_rest",
	core.CapabilityInversePublicREST:    "inverse_public_rest",
	core.CapabilityInverseTradingREST:   "inverse_trading_rest",
	core.CapabilityWebsocketPublic:      "websocket_public",
	core.CapabilityWebsocketPrivate:     "websocket_private",
	core.CapabilityTradingSpotAmend:     "trading_spot_amend",
	core.CapabilityTradingSpotCancel:    "trading_spot_cancel",
	core.CapabilityTradingLinearAmend:   "trading_linear_amend",
	core.CapabilityTradingLinearCancel:  "trading_linear_cancel",
	core.CapabilityTradingInverseAmend:  "trading_inverse_amend",
	core.CapabilityTradingInverseCancel: "trading_inverse_cancel",
	core.CapabilityAccountBalances:      "account_balances",
	core.CapabilityAccountPositions:     "account_positions",
	core.CapabilityAccountMargin:        "account_margin",
	core.CapabilityAccountTransfers:     "account_transfers",
	core.CapabilityMarketTrades:         "market_trades",
	core.CapabilityMarketTicker:         "market_ticker",
	core.CapabilityMarketOrderBook:      "market_orderbook",
	core.CapabilityMarketCandles:        "market_candles",
	core.CapabilityMarketFundingRates:   "market_funding_rates",
	core.CapabilityMarketMarkPrice:      "market_mark_price",
	core.CapabilityMarketLiquidations:   "market_liquidations",
}

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("capabilities", flag.ContinueOnError)
	fs.SetOutput(stderr)
	jsonOutput := fs.Bool("json", false, "emit JSON output")
	if err := fs.Parse(args); err != nil {
		return err
	}

	summaries := sharedplugin.Summaries()
	if len(summaries) == 0 {
		return fmt.Errorf("no exchanges registered")
	}

	rows := buildRows(summaries)
	if *jsonOutput {
		data, err := json.MarshalIndent(rows, "", "  ")
		if err != nil {
			return fmt.Errorf("encode JSON: %w", err)
		}
		if _, err := stdout.Write(data); err != nil {
			return err
		}
		_, err = stdout.Write([]byte("\n"))
		return err
	}

	return renderTable(stdout, rows)
}

func buildRows(summaries []sharedplugin.Summary) []matrixRow {
	rows := make([]matrixRow, 0, len(summaries))
	for _, summary := range summaries {
		caps := summary.Capabilities
		row := matrixRow{
			Name:         string(summary.Name),
			Protocol:     summary.ProtocolVersion,
			Categories:   evaluateCategories(caps),
			Capabilities: expandCapabilities(caps),
			Metadata:     cloneMetadata(summary.Metadata),
		}
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Name < rows[j].Name })
	return rows
}

func renderTable(w io.Writer, rows []matrixRow) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintf(tw, "Exchange\tProtocol")
	for _, cat := range matrixCategories {
		fmt.Fprintf(tw, "\t%s", cat.header)
	}
	fmt.Fprintln(tw)

	fmt.Fprintf(tw, "--------\t--------")
	for range matrixCategories {
		fmt.Fprintf(tw, "\t--------")
	}
	fmt.Fprintln(tw)

	for _, row := range rows {
		fmt.Fprintf(tw, "%s\t%s", row.Name, row.Protocol)
		for _, cat := range matrixCategories {
			mark := "✗"
			if row.Categories[cat.key] {
				mark = "✓"
			}
			fmt.Fprintf(tw, "\t%s", mark)
		}
		fmt.Fprintln(tw)
	}

	return tw.Flush()
}

func evaluateCategories(set core.ExchangeCapabilities) map[string]bool {
	result := make(map[string]bool, len(matrixCategories))
	for _, cat := range matrixCategories {
		result[cat.key] = hasAll(set, cat.all...)
	}
	return result
}

func expandCapabilities(set core.ExchangeCapabilities) []string {
	labels := make([]string, 0, len(orderedCapabilities))
	for _, cap := range orderedCapabilities {
		if set.Has(cap) {
			if label, ok := capabilityLabels[cap]; ok {
				labels = append(labels, label)
			}
		}
	}
	return labels
}

func hasAll(set core.ExchangeCapabilities, caps ...core.Capability) bool {
	if len(caps) == 0 {
		return false
	}
	return set.All(caps...)
}

func cloneMetadata(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func toCoreCaps(caps []exchangecap.Capability) []core.Capability {
	out := make([]core.Capability, 0, len(caps))
	for _, cap := range caps {
		out = append(out, core.Capability(cap))
	}
	return out
}
