package ws

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/coachpo/meltica/core"
	corews "github.com/coachpo/meltica/core/ws"
)

type okxPublicEnvelope struct {
	Arg struct {
		Channel string `json:"channel"`
		InstID  string `json:"instId"`
	} `json:"arg"`
	Action string            `json:"action"` // "snapshot" or "update"
	Data   []json.RawMessage `json:"data"`
	Event  string            `json:"event"`
	Msg    string            `json:"msg"`
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
	case channel == TopicTrade:
		return w.parseTradeSnapshot(msg, env.Data, instrument)
	case channel == TopicTicker:
		return w.parseTickerSnapshot(msg, env.Data, instrument)
	case strings.HasPrefix(channel, TopicBook):
		return w.parseBookData(msg, env.Data, instrument, env.Action)
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
	msg.Event = corews.TopicTrade
	msg.Parsed = &corews.TradeEvent{Symbol: instrument, Price: price, Quantity: qty, Time: when}
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
	msg.Event = corews.TopicTicker
	msg.Parsed = &corews.TickerEvent{Symbol: instrument, Bid: bid, Ask: ask, Time: when}
	return nil
}

func (w *WS) parseBookData(msg *core.Message, payload []json.RawMessage, instrument, action string) error {
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

	// Get or create order book for this symbol
	orderBook := w.orderBooks.GetOrCreateOrderBook(instrument)

	updateTime := parseMillis(rec.Ts)
	bids := depthLevelsFromPairs(rec.Bids)
	asks := depthLevelsFromPairs(rec.Asks)

	var success bool
	switch action {
	case "snapshot":
		// Full snapshot - replace entire order book
		orderBook.UpdateFromSnapshot(bids, asks, updateTime)
		orderBook.SetLastUpdateID(rec.SeqID)
		success = true
	case "update":
		// Incremental update - merge with existing order book
		success = UpdateFromOKXDelta(orderBook, bids, asks, rec.PrevSeqID, rec.SeqID, updateTime)
	default:
		// Unknown action, treat as snapshot
		orderBook.UpdateFromSnapshot(bids, asks, updateTime)
		orderBook.SetLastUpdateID(rec.SeqID)
		success = true
	}

	if !success {
		// If the update failed (sequence mismatch), skip this update
		// In a production system, you would request a fresh snapshot
		return nil
	}

	// Get the complete order book snapshot
	completeSnapshot := orderBook.GetSnapshot()

	msg.Event = corews.TopicBook
	msg.Parsed = &completeSnapshot
	return nil
}
