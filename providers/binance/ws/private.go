package ws

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/coachpo/meltica/core"
	"github.com/gorilla/websocket"
)

func (w *WS) SubscribePrivate(ctx context.Context, topics ...string) (core.Subscription, error) {
	// Binance private WS does not use topics; it is bound to listenKey
	key, err := w.p.CreateListenKey(ctx)
	if err != nil {
		return nil, err
	}
	base := "wss://stream.binance.com:9443/ws/" + key
	dialer := websocket.Dialer{HandshakeTimeout: 10 * time.Second}
	conn, _, err := dialer.DialContext(ctx, base, nil)
	if err != nil {
		return nil, err
	}
	sub := &wsSub{c: make(chan core.Message, 1024), err: make(chan error, 1), conn: conn}
	// keepalive ticker
	kaCtx, kaCancel := context.WithCancel(context.Background())
	go func() {
		t := time.NewTicker(30 * time.Minute)
		defer t.Stop()
		for {
			select {
			case <-kaCtx.Done():
				return
			case <-t.C:
				_ = w.p.KeepAliveListenKey(context.Background(), key)
			}
		}
	}()
	go func() {
		defer close(sub.c)
		defer close(sub.err)
		defer kaCancel()
		for {
			_, data, e := conn.ReadMessage()
			if e != nil {
				sub.err <- e
				return
			}
			// Try to detect execution/balance events
			var env struct {
				E string `json:"e"`
			}
			msg := core.Message{Topic: "", Raw: data, At: time.Now()}
			if json.Unmarshal(data, &env) == nil && env.E == "ORDER_TRADE_UPDATE" {
				// See Binance User Data Streams v3 order update schema
				var ou struct {
					E int64 `json:"E"`
					O struct {
						S  string `json:"s"`
						I  int64  `json:"i"`
						X  string `json:"X"`
						Z  string `json:"z"`
						Ap string `json:"ap"`
						T  int64  `json:"T"`
					} `json:"o"`
				}
				_ = json.Unmarshal(data, &ou)
				filled, _ := parseDecimalToRat(ou.O.Z)
				avg, _ := parseDecimalToRat(ou.O.Ap)
				msg.Event = "order"
				msg.Parsed = &core.OrderEvent{
					Symbol:    ou.O.S,
					OrderID:   fmt.Sprintf("%d", ou.O.I),
					Status:    mapBStatus(ou.O.X),
					FilledQty: filled,
					AvgPrice:  avg,
					Time:      time.UnixMilli(ou.O.T),
				}
			} else if env.E == "outboundAccountPosition" || env.E == "balanceUpdate" {
				var be core.BalanceEvent
				// outboundAccountPosition: array balances
				var oap struct {
					E int64 `json:"E"`
					B []struct {
						A string `json:"a"`
						F string `json:"f"`
					} `json:"B"`
				}
				if json.Unmarshal(data, &oap) == nil && len(oap.B) > 0 {
					for _, b := range oap.B {
						amt, _ := parseDecimalToRat(b.F)
						be.Balances = append(be.Balances, core.Balance{Asset: b.A, Available: amt, Time: time.UnixMilli(oap.E)})
					}
					msg.Event = "balance"
					msg.Parsed = &be
				} else {
					var bu struct {
						E int64  `json:"E"`
						A string `json:"a"`
						D string `json:"d"`
					}
					if json.Unmarshal(data, &bu) == nil && bu.A != "" {
						amt, _ := parseDecimalToRat(bu.D)
						be.Balances = append(be.Balances, core.Balance{Asset: bu.A, Available: amt, Time: time.UnixMilli(bu.E)})
						msg.Event = "balance"
						msg.Parsed = &be
					}
				}
			}
			sub.c <- msg
		}
	}()
	return &wsPrivWrapper{sub, key, w.p, kaCancel}, nil
}

type wsPrivWrapper struct {
	inner  *wsSub
	key    string
	p      Provider
	kaStop context.CancelFunc
}

func (w *wsPrivWrapper) C() <-chan core.Message { return w.inner.C() }
func (w *wsPrivWrapper) Err() <-chan error      { return w.inner.Err() }
func (w *wsPrivWrapper) Close() error {
	w.kaStop()
	_ = w.p.CloseListenKey(context.Background(), w.key)
	return w.inner.Close()
}
