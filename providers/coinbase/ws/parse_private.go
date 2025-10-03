package ws

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/coachpo/meltica/core"
	corews "github.com/coachpo/meltica/core/ws"
)

func (w *WS) parsePrivateMessage(msg *core.Message, payload []byte) error {
	var env map[string]any
	if err := json.Unmarshal(payload, &env); err != nil {
		return err
	}
	typeVal := fmt.Sprint(env["type"])
	switch typeVal {
	case "subscriptions", "heartbeat", "error":
		msg.Topic = typeVal
		msg.Event = typeVal
		return nil
	case "received", "open", "done", "change", "activate":
		return w.parseOrderEvent(msg, typeVal, env)
	case "wallet", "profile":
		return w.parseBalance(msg, typeVal, env)
	default:
		msg.Topic = typeVal
		msg.Event = typeVal
		return nil
	}
}

func (w *WS) parseOrderEvent(msg *core.Message, event string, env map[string]any) error {
	symbol := w.canonicalSymbol(fmt.Sprint(env["product_id"]))
	id := fmt.Sprint(env["order_id"]) + fmt.Sprint(env["client_oid"])
	status := mapStatus(fmt.Sprint(env["type"]), fmt.Sprint(env["reason"]))
	filled := parseDecimal(fmt.Sprint(env["filled_size"]))
	avg := parseDecimal(fmt.Sprint(env["price"]))
	msg.Topic = topicFromProviderName(event, symbol)
	msg.Event = corews.TopicUserOrder
	msg.Parsed = &corews.OrderEvent{Symbol: symbol, OrderID: id, Status: status, FilledQty: filled, AvgPrice: avg, Time: parseTime(fmt.Sprint(env["time"]))}
	return nil
}

func (w *WS) parseBalance(msg *core.Message, event string, env map[string]any) error {
	accounts, _ := env["accounts"].([]any)
	balances := make([]corews.Balance, 0, len(accounts))
	for _, acct := range accounts {
		row, _ := acct.(map[string]any)
		asset := strings.ToUpper(fmt.Sprint(row["currency"]))
		free := parseDecimal(fmt.Sprint(row["balance"]))
		balances = append(balances, corews.Balance{Asset: asset, Available: free, Time: parseTime(fmt.Sprint(env["time"]))})
	}
	msg.Topic = topicFromProviderName(event, "")
	msg.Event = corews.TopicUserBalance
	msg.Parsed = &corews.BalanceEvent{Balances: balances}
	return nil
}
