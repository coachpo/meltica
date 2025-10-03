package ws

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/coachpo/meltica/core"
	corews "github.com/coachpo/meltica/core/ws"
)

func (w *WS) parsePrivate(msg *core.Message, data []byte, topics []string) error {
	if len(data) == 0 {
		return nil
	}
	if data[0] == '[' {
		var arr []any
		if err := json.Unmarshal(data, &arr); err != nil {
			return err
		}
		if len(arr) < 4 {
			return nil
		}
		channelName, _ := arr[len(arr)-2].(string)
		payload := arr[2]
		switch channelName {
		case string(KRKTopicOwnTrades):
			return w.parseOwnTrades(msg, payload)
		case string(KRKTopicOpenOrders):
			return w.parseOpenOrders(msg, payload)
		}
		return nil
	}
	var env map[string]any
	if err := json.Unmarshal(data, &env); err != nil {
		return err
	}
	msg.Topic = core.Topic(fmt.Sprint(env["event"]))
	return nil
}

func (w *WS) parseOwnTrades(msg *core.Message, payload any) error {
	trades, ok := payload.(map[string]any)
	if !ok {
		return nil
	}
	for id, raw := range trades {
		rec, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		evt := w.orderEventFromTrade(id, rec)
		if evt == nil {
			continue
		}
		msg.Topic = corews.UserOrderTopic(evt.Symbol)
		msg.Event = corews.EventOrderUpdate
		msg.Parsed = evt
	}
	return nil
}

func (w *WS) parseOpenOrders(msg *core.Message, payload any) error {
	recMap, ok := payload.(map[string]any)
	if !ok {
		return nil
	}
	for id, raw := range recMap {
		rec, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		evt := w.orderEventFromOpenOrder(id, rec)
		if evt == nil {
			continue
		}
		msg.Topic = corews.UserOrderTopic(evt.Symbol)
		msg.Event = corews.EventOrderUpdate
		msg.Parsed = evt
	}
	return nil
}

func (w *WS) orderEventFromTrade(id string, rec map[string]any) *corews.OrderEvent {
	canon := w.p.MapNativeToCanon(strings.ToUpper(fmt.Sprint(rec["pair"])))
	filled := parseDecimalStr(fmt.Sprint(rec["vol"]))
	avg := parseDecimalStr(fmt.Sprint(rec["price"]))
	status := mapStatus(fmt.Sprint(rec["status"]))
	return &corews.OrderEvent{
		Symbol:    canon,
		OrderID:   id,
		Status:    status,
		FilledQty: filled,
		AvgPrice:  avg,
		Time:      parseTradesTS(fmt.Sprint(rec["time"])),
	}
}

func (w *WS) orderEventFromOpenOrder(id string, rec map[string]any) *corews.OrderEvent {
	descr, _ := rec["descr"].(map[string]any)
	pair := ""
	if descr != nil {
		pair = fmt.Sprint(descr["pair"])
	}
	canon := w.p.MapNativeToCanon(strings.ToUpper(pair))
	status := mapStatus(fmt.Sprint(rec["status"]))
	priceMap, _ := rec["price"].(map[string]any)
	avg := parseDecimalStr(fmt.Sprint(priceMap["last"]))
	filled := parseDecimalStr(fmt.Sprint(rec["vol_exec"]))
	return &corews.OrderEvent{
		Symbol:    canon,
		OrderID:   id,
		Status:    status,
		FilledQty: filled,
		AvgPrice:  avg,
	}
}
