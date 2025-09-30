package test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/coachpo/meltica/providers/binance"
)

func TestGolden_TickerBookTicker(t *testing.T) {
	data := []byte(`{"bidPrice":"12345.67000000","askPrice":"12346.12000000"}`)
	var resp struct {
		Bid string `json:"bidPrice"`
		Ask string `json:"askPrice"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatal(err)
	}
	bid, _ := binance.ParseDecimalToRat(resp.Bid)
	ask, _ := binance.ParseDecimalToRat(resp.Ask)
	got := map[string]string{"bid": bid.RatString(), "ask": ask.RatString()}
	wantPath := filepath.Join("testdata", "binance_bookTicker_golden.json")
	want, err := os.ReadFile(wantPath)
	if err != nil {
		// write the golden if missing
		_ = os.MkdirAll(filepath.Dir(wantPath), 0o755)
		b, _ := json.MarshalIndent(got, "", "  ")
		if err := os.WriteFile(wantPath, b, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	var wantObj map[string]string
	if err := json.Unmarshal(want, &wantObj); err != nil {
		t.Fatal(err)
	}
	if got["bid"] != wantObj["bid"] || got["ask"] != wantObj["ask"] {
		b, _ := json.MarshalIndent(got, "", "  ")
		t.Fatalf("mismatch\n got=%s\nwant=%s", string(b), string(want))
	}
}
