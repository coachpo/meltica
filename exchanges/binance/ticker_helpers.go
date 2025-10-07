package binance

import (
	"strings"
	"time"

	"github.com/coachpo/meltica/core"
	"github.com/coachpo/meltica/internal/numeric"
)

type tickerResponse struct {
	BidPrice string `json:"bidPrice"`
	AskPrice string `json:"askPrice"`
}

var tickerNow = time.Now

func buildTicker(symbol, bidPrice, askPrice string) core.Ticker {
	bid, _ := numeric.Parse(strings.TrimSpace(bidPrice))
	ask, _ := numeric.Parse(strings.TrimSpace(askPrice))
	return core.Ticker{Symbol: symbol, Bid: bid, Ask: ask, Time: tickerNow()}
}
