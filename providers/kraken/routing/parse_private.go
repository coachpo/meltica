package routing

import (
	"encoding/json"
	"fmt"
	"strings"

	coreprovider "github.com/coachpo/meltica/core/provider"
)

func (w *WSRouter) parsePrivateMessage(msg *RoutedMessage, data []byte) error {
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
		case KrkTopicOwnTrades:
			return w.parseOwnTrades(msg, payload)
		case KrkTopicOpenOrders:
			return w.parseOpenOrders(msg, payload)
		}
		return nil
	}
	var env map[string]any
	if err := json.Unmarshal(data, &env); err != nil {
		return err
	}
	msg.Topic = fmt.Sprint(env["event"])
	return nil
}

func (w *WSRouter) parseOwnTrades(msg *RoutedMessage, payload any) error {
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
		msg.Topic = userOrderTopic(evt.Symbol)
		msg.Route = coreprovider.RouteOrderUpdate
		msg.Parsed = evt
	}
	return nil
}

func (w *WSRouter) parseOpenOrders(msg *RoutedMessage, payload any) error {
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
		msg.Topic = userOrderTopic(evt.Symbol)
		msg.Route = coreprovider.RouteOrderUpdate
		msg.Parsed = evt
	}
	return nil
}

func (w *WSRouter) orderEventFromTrade(id string, rec map[string]any) *coreprovider.OrderEvent {
	canon := w.deps.MapNativeToCanon(strings.ToUpper(fmt.Sprint(rec["pair"])))
	filled := parseDecimalStr(fmt.Sprint(rec["vol"]))
	avg := parseDecimalStr(fmt.Sprint(rec["price"]))
	status := mapStatus(fmt.Sprint(rec["status"]))
	return &coreprovider.OrderEvent{
		Symbol:    canon,
		OrderID:   id,
		Status:    status,
		FilledQty: filled,
		AvgPrice:  avg,
		Time:      parseTradesTS(fmt.Sprint(rec["time"])),
	}
}

func (w *WSRouter) orderEventFromOpenOrder(id string, rec map[string]any) *coreprovider.OrderEvent {
	descr, _ := rec["descr"].(map[string]any)
	pair := ""
	if descr != nil {
		pair = fmt.Sprint(descr["pair"])
	}
	canon := w.deps.MapNativeToCanon(strings.ToUpper(pair))
	status := mapStatus(fmt.Sprint(rec["status"]))
	priceMap, _ := rec["price"].(map[string]any)
	avg := parseDecimalStr(fmt.Sprint(priceMap["last"]))
	filled := parseDecimalStr(fmt.Sprint(rec["vol_exec"]))
	return &coreprovider.OrderEvent{
		Symbol:    canon,
		OrderID:   id,
		Status:    status,
		FilledQty: filled,
		AvgPrice:  avg,
	}
}

func userOrderTopic(symbol string) string {
	if symbol == "" {
		return KrkTopicOpenOrders
	}
	return KrkTopicOpenOrders + ":" + symbol
}
