package test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	okx "github.com/coachpo/meltica/providers/okx"
)

func TestGolden_OKX_Ticker(t *testing.T) {
	data := []byte(`{"data":[{"bidPx":"27123.45","askPx":"27124.01"}]}`)
	var resp struct {
		Data []struct {
			BidPx string `json:"bidPx"`
			AskPx string `json:"askPx"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatal(err)
	}
	bid, _ := okx.ParseDecimalToRat(resp.Data[0].BidPx)
	ask, _ := okx.ParseDecimalToRat(resp.Data[0].AskPx)
	got := map[string]string{"bid": bid.RatString(), "ask": ask.RatString()}
	wantPath := filepath.Join("testdata", "okx_ticker_golden.json")
	want, err := os.ReadFile(wantPath)
	if err != nil {
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
