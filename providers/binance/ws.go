package binance

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/coachpo/meltica/core"
	"github.com/gorilla/websocket"
)

type wsSub struct {
	c    chan core.Message
	err  chan error
	conn *websocket.Conn
}

func (s *wsSub) C() <-chan core.Message { return s.c }
func (s *wsSub) Err() <-chan error      { return s.err }
func (s *wsSub) Close() error {
	if s.conn != nil {
		return s.conn.Close()
	}
	return nil
}

func (w ws) SubscribePublic(ctx context.Context, topics ...string) (core.Subscription, error) {
	if len(topics) == 0 {
		return nil, fmt.Errorf("no topics")
	}
	// Translate canonical topics "channel:BASE-QUOTE" to binance streams
	streams := make([]string, 0, len(topics))
	for _, t := range topics {
		ch := t
		inst := ""
		if i := strings.IndexByte(t, ':'); i > 0 {
			ch = t[:i]
			inst = t[i+1:]
		}
		if inst != "" {
			bSym := strings.ToLower(core.CanonicalToBinance(inst))
			switch ch {
			case "trades":
				streams = append(streams, bSym+"@trade")
			case "tickers":
				streams = append(streams, bSym+"@bookTicker")
			case "books5", "depth5", "depth":
				streams = append(streams, bSym+"@depth5")
			default:
				streams = append(streams, t)
			}
		} else {
			streams = append(streams, t)
		}
	}
	// Build combined stream URL
	base := "wss://stream.binance.com:9443/stream"
	q := url.Values{}
	q.Set("streams", strings.Join(streams, "/"))
	u := base + "?" + q.Encode()
	dialer := websocket.Dialer{HandshakeTimeout: 10 * time.Second}
	conn, _, err := dialer.DialContext(ctx, u, nil)
	if err != nil {
		return nil, err
	}
	sub := &wsSub{c: make(chan core.Message, 1024), err: make(chan error, 1), conn: conn}
	go func() {
		defer close(sub.c)
		defer close(sub.err)
		for {
			_, data, e := conn.ReadMessage()
			if e != nil {
				sub.err <- e
				return
			}
			// Try to parse combined stream envelope
			var env struct {
				Stream string          `json:"stream"`
				Data   json.RawMessage `json:"data"`
			}
			msg := core.Message{Topic: "", Raw: data, At: time.Now()}
			payload := data
			if json.Unmarshal(data, &env) == nil && len(env.Data) > 0 {
				msg.Topic = env.Stream
				payload = env.Data
			}
			// Heuristic parse for trade/ticker/depth
			var probe map[string]any
			if json.Unmarshal(payload, &probe) == nil {
				if _, ok := probe["p"]; ok { // trade
					var tr struct {
						S string `json:"s"`
						P string `json:"p"`
						Q string `json:"q"`
						T int64  `json:"T"`
					}
					_ = json.Unmarshal(payload, &tr)
					px, _ := parseDecimalToRat(tr.P)
					qty, _ := parseDecimalToRat(tr.Q)
					msg.Event = "trade"
					msg.Parsed = &core.TradeEvent{Symbol: tr.S, Price: px, Quantity: qty, Time: time.UnixMilli(tr.T)}
				} else if _, okb := probe["b"]; okb { // bookTicker or depth
					// check for bookTicker: has keys b (bid price) and a (ask price) as strings and s symbol
					if _, ok := probe["s"]; ok && (hasString(probe, "b") && hasString(probe, "a")) {
						var bt struct {
							S string `json:"s"`
							B string `json:"b"`
							A string `json:"a"`
							E int64  `json:"E"`
						}
						_ = json.Unmarshal(payload, &bt)
						bid, _ := parseDecimalToRat(bt.B)
						ask, _ := parseDecimalToRat(bt.A)
						msg.Event = "ticker"
						msg.Parsed = &core.TickerEvent{Symbol: bt.S, Bid: bid, Ask: ask, Time: time.UnixMilli(bt.E)}
					} else {
						// depth update: b and a arrays
						var du struct {
							S    string     `json:"s"`
							E    int64      `json:"E"`
							Bids [][]string `json:"b"`
							Asks [][]string `json:"a"`
						}
						_ = json.Unmarshal(payload, &du)
						de := core.DepthEvent{Symbol: du.S, Time: time.UnixMilli(du.E)}
						for _, lv := range du.Bids {
							if len(lv) < 2 {
								continue
							}
							p, _ := parseDecimalToRat(lv[0])
							q, _ := parseDecimalToRat(lv[1])
							de.Bids = append(de.Bids, core.DepthLevel{Price: p, Qty: q})
						}
						for _, lv := range du.Asks {
							if len(lv) < 2 {
								continue
							}
							p, _ := parseDecimalToRat(lv[0])
							q, _ := parseDecimalToRat(lv[1])
							de.Asks = append(de.Asks, core.DepthLevel{Price: p, Qty: q})
						}
						msg.Event = "depth"
						msg.Parsed = &de
					}
				}
			}
			sub.c <- msg
		}
	}()
	return sub, nil
}

// Private subscription implemented in ws_private.go
