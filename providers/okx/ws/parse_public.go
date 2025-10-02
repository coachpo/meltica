package ws

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/coachpo/meltica/core"
)

type okxPublicEnvelope struct {
	Arg struct {
		Channel string `json:"channel"`
		InstID  string `json:"instId"`
	} `json:"arg"`
	Data  []json.RawMessage `json:"data"`
	Event string            `json:"event"`
	Msg   string            `json:"msg"`
}

func (w *WS) parsePublicMessage(msg *core.Message, raw []byte) error {
	var env okxPublicEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil
	}
	if len(env.Data) == 0 {
		msg.Topic = env.Event
		msg.Event = env.Event
		if env.Msg != "" {
			msg.Parsed = env.Msg
		}
		return nil
	}
	channel := env.Arg.Channel
	instrument := env.Arg.InstID
	msg.Topic = topicFromChannel(channel, instrument)

	switch {
	case channel == "trades":
		return w.parseTradeSnapshot(msg, env.Data, instrument)
	case channel == "tickers":
		return w.parseTickerSnapshot(msg, env.Data, instrument)
	case strings.HasPrefix(channel, "books"):
		return w.parseBookSnapshot(msg, env.Data, instrument)
	default:
		return nil
	}
}

func (w *WS) parseTradeSnapshot(msg *core.Message, payload []json.RawMessage, instrument string) error {
	if len(payload) == 0 {
		return nil
	}
	var rec struct {
		Px string `json:"px"`
		Sz string `json:"sz"`
		Ts string `json:"ts"`
	}
	if err := json.Unmarshal(payload[len(payload)-1], &rec); err != nil {
		return err
	}
	price, _ := parseDecimalToRat(rec.Px)
	qty, _ := parseDecimalToRat(rec.Sz)
	when := parseMillis(rec.Ts)
	if when.IsZero() {
		when = time.Now()
	}
	msg.Event = "trade"
	msg.Parsed = &core.TradeEvent{Symbol: instrument, Price: price, Quantity: qty, Time: when}
	return nil
}

func (w *WS) parseTickerSnapshot(msg *core.Message, payload []json.RawMessage, instrument string) error {
	if len(payload) == 0 {
		return nil
	}
	var rec struct {
		BidPx string `json:"bidPx"`
		AskPx string `json:"askPx"`
		Ts    string `json:"ts"`
	}
	if err := json.Unmarshal(payload[len(payload)-1], &rec); err != nil {
		return err
	}
	bid, _ := parseDecimalToRat(rec.BidPx)
	ask, _ := parseDecimalToRat(rec.AskPx)
	when := parseMillis(rec.Ts)
	if when.IsZero() {
		when = time.Now()
	}
	msg.Event = "ticker"
	msg.Parsed = &core.TickerEvent{Symbol: instrument, Bid: bid, Ask: ask, Time: when}
	return nil
}

func (w *WS) parseBookSnapshot(msg *core.Message, payload []json.RawMessage, instrument string) error {
	if len(payload) == 0 {
		return nil
	}
	var rec struct {
		Bids [][]string `json:"bids"`
		Asks [][]string `json:"asks"`
		Ts   string     `json:"ts"`
	}
	if err := json.Unmarshal(payload[len(payload)-1], &rec); err != nil {
		return err
	}
	de := core.DepthEvent{Symbol: instrument, Time: parseMillis(rec.Ts)}
	de.Bids = append(de.Bids, depthLevelsFromPairs(rec.Bids)...)
	de.Asks = append(de.Asks, depthLevelsFromPairs(rec.Asks)...)
	msg.Event = "depth"
	msg.Parsed = &de
	return nil
}
