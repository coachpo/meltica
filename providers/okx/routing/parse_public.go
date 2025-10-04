package routing

import (
	"encoding/json"
	"strings"
	"time"

	coreprovider "github.com/coachpo/meltica/core/provider"
)

type okxPublicEnvelope struct {
	Arg struct {
		Channel string `json:"channel"`
		InstID  string `json:"instId"`
	} `json:"arg"`
	Action string            `json:"action"`
	Data   []json.RawMessage `json:"data"`
	Event  string            `json:"event"`
	Msg    string            `json:"msg"`
}

func (w *WSRouter) parsePublicMessage(msg *coreprovider.RoutedMessage, raw []byte) error {
	var env okxPublicEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil
	}
	if len(env.Data) == 0 {
		msg.Topic = env.Event
		msg.Route = coreprovider.RouteUnknown
		if env.Msg != "" {
			msg.Parsed = env.Msg
		}
		return nil
	}
	channel := env.Arg.Channel
	instrument := env.Arg.InstID
	if instrument == "" {
		return nil
	}
	symbol := instrument
	msg.Topic = topicFromChannel(channel, symbol)

	switch {
	case channel == OKXTopicTrade:
		return w.parseTradeSnapshot(msg, env.Data, symbol)
	case channel == OKXTopicTicker:
		return w.parseTickerSnapshot(msg, env.Data, symbol)
	case strings.HasPrefix(channel, OKXTopicBook):
		return w.parseBookData(msg, env.Data, symbol, env.Action)
	default:
		return nil
	}
}

func (w *WSRouter) parseTradeSnapshot(msg *coreprovider.RoutedMessage, payload []json.RawMessage, symbol string) error {
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
	msg.Route = coreprovider.RouteTradeUpdate
	msg.Parsed = &coreprovider.TradeEvent{Symbol: symbol, Price: price, Quantity: qty, Time: when}
	return nil
}

func (w *WSRouter) parseTickerSnapshot(msg *coreprovider.RoutedMessage, payload []json.RawMessage, symbol string) error {
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
	msg.Route = coreprovider.RouteTickerUpdate
	msg.Parsed = &coreprovider.TickerEvent{Symbol: symbol, Bid: bid, Ask: ask, Time: when}
	return nil
}

func (w *WSRouter) parseBookData(msg *coreprovider.RoutedMessage, payload []json.RawMessage, symbol, action string) error {
	if len(payload) == 0 {
		return nil
	}

	var rec struct {
		Bids      [][]string `json:"bids"`
		Asks      [][]string `json:"asks"`
		Ts        string     `json:"ts"`
		Checksum  int64      `json:"checksum"`
		PrevSeqID int64      `json:"prevSeqId"`
		SeqID     int64      `json:"seqId"`
	}

	if err := json.Unmarshal(payload[len(payload)-1], &rec); err != nil {
		return err
	}

	orderBook := w.orderBooks.GetOrCreateOrderBook(symbol)
	updateTime := parseMillis(rec.Ts)
	bids := depthLevelsFromPairs(rec.Bids)
	asks := depthLevelsFromPairs(rec.Asks)

	var success bool
	switch action {
	case "snapshot":
		orderBook.UpdateFromSnapshot(bids, asks, updateTime)
		orderBook.SetLastUpdateID(rec.SeqID)
		success = true
	case "update":
		success = UpdateFromOKXDelta(orderBook, bids, asks, rec.PrevSeqID, rec.SeqID, updateTime)
	default:
		orderBook.UpdateFromSnapshot(bids, asks, updateTime)
		orderBook.SetLastUpdateID(rec.SeqID)
		success = true
	}

	if !success {
		return nil
	}

	snapshot := orderBook.GetSnapshot()
	msg.Route = coreprovider.RouteBookSnapshot
	msg.Parsed = &snapshot
	return nil
}
