package okx

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
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
	url := "wss://ws.okx.com:8443/ws/v5/public"
	dialer := websocket.Dialer{HandshakeTimeout: 10 * time.Second}
	conn, _, err := dialer.DialContext(ctx, url, nil)
	if err != nil {
		return nil, err
	}

	// OKX requires a subscribe message
	payload := struct {
		Op   string              `json:"op"`
		Args []map[string]string `json:"args"`
	}{Op: "subscribe", Args: []map[string]string{}}
	for _, t := range topics {
		ch := t
		inst := ""
		if i := strings.IndexByte(t, ':'); i > 0 {
			ch = t[:i]
			inst = t[i+1:]
		}
		arg := map[string]string{"channel": ch}
		if inst != "" {
			arg["instId"] = inst
		}
		payload.Args = append(payload.Args, arg)
	}
	if err := conn.WriteJSON(payload); err != nil {
		_ = conn.Close()
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
			// OKX WS envelope: {arg:{channel,instId}, data:[...]} or event messages
			var env struct {
				Arg struct {
					Channel string `json:"channel"`
					InstId  string `json:"instId"`
				} `json:"arg"`
				Data []json.RawMessage `json:"data"`
			}
			msg := core.Message{Topic: "", Raw: data, At: time.Now()}
			if json.Unmarshal(data, &env) == nil && len(env.Data) > 0 {
				ch := env.Arg.Channel
				inst := env.Arg.InstId
				if len(env.Data) > 0 {
					payload := env.Data[0]
					// trades
					if ch == "trades" {
						var tr struct {
							InstId string `json:"instId"`
							Px     string `json:"px"`
							Sz     string `json:"sz"`
							Ts     string `json:"ts"`
						}
						_ = json.Unmarshal(payload, &tr)
						px, _ := parseDecimalToRat(tr.Px)
						sz, _ := parseDecimalToRat(tr.Sz)
						// ts is millis string
						tms, _ := time.ParseDuration(tr.Ts + "ms")
						msg.Event = "trade"
						msg.Parsed = &core.TradeEvent{Symbol: inst, Price: px, Quantity: sz, Time: time.Unix(0, tms.Nanoseconds())}
					} else if ch == "tickers" {
						var tk struct {
							BidPx string `json:"bidPx"`
							AskPx string `json:"askPx"`
							Ts    string `json:"ts"`
						}
						_ = json.Unmarshal(payload, &tk)
						bid, _ := parseDecimalToRat(tk.BidPx)
						ask, _ := parseDecimalToRat(tk.AskPx)
						tms, _ := time.ParseDuration(tk.Ts + "ms")
						msg.Event = "ticker"
						msg.Parsed = &core.TickerEvent{Symbol: inst, Bid: bid, Ask: ask, Time: time.Unix(0, tms.Nanoseconds())}
					} else if strings.HasPrefix(ch, "books") { // books, books5, etc
						var bk struct {
							Bids [][]string `json:"bids"`
							Asks [][]string `json:"asks"`
							Ts   string     `json:"ts"`
						}
						_ = json.Unmarshal(payload, &bk)
						de := core.DepthEvent{Symbol: inst}
						if bk.Ts != "" {
							if d, _ := time.ParseDuration(bk.Ts + "ms"); d > 0 {
								de.Time = time.Unix(0, d.Nanoseconds())
							}
						}
						for _, lv := range bk.Bids {
							if len(lv) < 2 {
								continue
							}
							p, _ := parseDecimalToRat(lv[0])
							q, _ := parseDecimalToRat(lv[1])
							de.Bids = append(de.Bids, core.DepthLevel{Price: p, Qty: q})
						}
						for _, lv := range bk.Asks {
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

func (w ws) SubscribePrivate(ctx context.Context, topics ...string) (core.Subscription, error) {
	url := "wss://ws.okx.com:8443/ws/v5/private"
	dialer := websocket.Dialer{HandshakeTimeout: 10 * time.Second}
	conn, _, err := dialer.DialContext(ctx, url, nil)
	if err != nil {
		return nil, err
	}
	// login
	ts := time.Now().UTC().Format(time.RFC3339)
	pre := ts + "GET" + "/users/self/verify"
	mac := hmac.New(sha256.New, []byte(w.p.secret))
	mac.Write([]byte(pre))
	sign := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	login := map[string]any{
		"op": "login",
		"args": []map[string]string{{
			"apiKey":     w.p.apiKey,
			"passphrase": w.p.passphrase,
			"timestamp":  ts,
			"sign":       sign,
		}},
	}
	if err := conn.WriteJSON(login); err != nil {
		_ = conn.Close()
		return nil, err
	}
	// Optionally subscribe to channels (e.g., orders:BTC-USDT)
	if len(topics) > 0 {
		payload := struct {
			Op   string              `json:"op"`
			Args []map[string]string `json:"args"`
		}{Op: "subscribe", Args: []map[string]string{}}
		for _, t := range topics {
			ch := t
			inst := ""
			if i := strings.IndexByte(t, ':'); i > 0 {
				ch = t[:i]
				inst = t[i+1:]
			}
			arg := map[string]string{"channel": ch}
			if inst != "" {
				arg["instId"] = inst
			}
			payload.Args = append(payload.Args, arg)
		}
		if err := conn.WriteJSON(payload); err != nil {
			_ = conn.Close()
			return nil, err
		}
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
			// Parse orders private events
			var env struct {
				Arg struct {
					Channel string `json:"channel"`
					InstId  string `json:"instId"`
				} `json:"arg"`
				Data []json.RawMessage `json:"data"`
			}
			msg := core.Message{Topic: "", Raw: data, At: time.Now()}
			if json.Unmarshal(data, &env) == nil && len(env.Data) > 0 && env.Arg.Channel == "orders" {
				var ord struct {
					OrdId     string `json:"ordId"`
					InstId    string `json:"instId"`
					State     string `json:"state"`
					AccFillSz string `json:"accFillSz"`
					AvgPx     string `json:"avgPx"`
					Ts        string `json:"ts"`
				}
				_ = json.Unmarshal(env.Data[0], &ord)
				filled, _ := parseDecimalToRat(ord.AccFillSz)
				avg, _ := parseDecimalToRat(ord.AvgPx)
				var t time.Time
				if ord.Ts != "" {
					if d, _ := time.ParseDuration(ord.Ts + "ms"); d > 0 {
						t = time.Unix(0, d.Nanoseconds())
					}
				}
				msg.Event = "order"
				msg.Parsed = &core.OrderEvent{Symbol: ord.InstId, OrderID: ord.OrdId, Status: mapOKXStatus(ord.State), FilledQty: filled, AvgPrice: avg, Time: t}
			} else if len(env.Data) > 0 && env.Arg.Channel == "balance_and_position" {
				var be core.BalanceEvent
				var bal struct {
					Data []struct {
						BalData []struct {
							Ccy     string `json:"ccy"`
							CashBal string `json:"cashBal"`
						} `json:"balData"`
					} `json:"data"`
				}
				_ = json.Unmarshal(data, &bal)
				for _, d := range bal.Data {
					for _, b := range d.BalData {
						amt, _ := parseDecimalToRat(b.CashBal)
						be.Balances = append(be.Balances, core.Balance{Asset: b.Ccy, Available: amt, Time: time.Now()})
					}
				}
				msg.Event = "balance"
				msg.Parsed = &be
			}
			sub.c <- msg
		}
	}()
	return sub, nil
}
