package binance

import (
	"encoding/json"
	"testing"
	"time"

	numeric "github.com/coachpo/meltica/exchanges/shared/infra/numeric"
)

func TestBuildTickerFromBidAsk(t *testing.T) {
	// freeze time for deterministic assertions
	originalNow := tickerNow
	fixed := time.Unix(1700000000, 0)
	tickerNow = func() time.Time { return fixed }
	defer func() { tickerNow = originalNow }()

	cases := []struct {
		name    string
		symbol  string
		payload string
		wantBid string
		wantAsk string
	}{
		{
			name:    "spot snapshot",
			symbol:  "BTC-USDT",
			payload: `{"bidPrice":"35210.12","askPrice":"35210.45"}`,
			wantBid: "35210.12",
			wantAsk: "35210.45",
		},
		{
			name:    "futures snapshot",
			symbol:  "ETH-USDT",
			payload: `{"bidPrice":"2110.5","askPrice":"2110.9"}`,
			wantBid: "2110.5",
			wantAsk: "2110.9",
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			var resp tickerResponse
			if err := json.Unmarshal([]byte(tt.payload), &resp); err != nil {
				t.Fatalf("failed to unmarshal payload: %v", err)
			}

			ticker := buildTicker(tt.symbol, resp.BidPrice, resp.AskPrice)

			wantBid, _ := numeric.Parse(tt.wantBid)
			wantAsk, _ := numeric.Parse(tt.wantAsk)

			if ticker.Symbol != tt.symbol {
				t.Fatalf("expected symbol %q, got %q", tt.symbol, ticker.Symbol)
			}
			if (ticker.Bid == nil) != (wantBid == nil) || (ticker.Bid != nil && ticker.Bid.Cmp(wantBid) != 0) {
				t.Fatalf("unexpected bid: got %v want %v", ticker.Bid, wantBid)
			}
			if (ticker.Ask == nil) != (wantAsk == nil) || (ticker.Ask != nil && ticker.Ask.Cmp(wantAsk) != 0) {
				t.Fatalf("unexpected ask: got %v want %v", ticker.Ask, wantAsk)
			}
			if !ticker.Time.Equal(fixed) {
				t.Fatalf("expected time %v, got %v", fixed, ticker.Time)
			}
		})
	}
}
