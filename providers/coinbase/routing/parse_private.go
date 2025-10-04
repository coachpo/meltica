package routing

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/coachpo/meltica/core"
	coreprovider "github.com/coachpo/meltica/core/provider"
)

func (w *WSRouter) parsePrivateMessage(msg *coreprovider.RoutedMessage, payload []byte) error {
	var env map[string]any
	if err := json.Unmarshal(payload, &env); err != nil {
		return err
	}
	typeVal := fmt.Sprint(env["type"])
	switch typeVal {
	case "subscriptions", "heartbeat", "error":
		msg.Topic = typeVal
		msg.Route = coreprovider.RouteUnknown
		return nil
	case "received", "open", "done", "change", "activate":
		return w.parseOrderEvent(msg, typeVal, env)
	case "wallet", "profile":
		return w.parseBalance(msg, typeVal, env)
	default:
		msg.Topic = typeVal
		msg.Route = coreprovider.RouteUnknown
		return nil
	}
}

func (w *WSRouter) parseOrderEvent(msg *coreprovider.RoutedMessage, event string, env map[string]any) error {
	symbol := w.deps.CanonicalFromNative(fmt.Sprint(env["product_id"]))
	id := fmt.Sprint(env["order_id"]) + fmt.Sprint(env["client_oid"])
	status := mapStatus(fmt.Sprint(env["type"]), fmt.Sprint(env["reason"]))
	filled := parseDecimal(fmt.Sprint(env["filled_size"]))
	avg := parseDecimal(fmt.Sprint(env["price"]))
	msg.Topic = topicFromProviderName(event, symbol)
	msg.Route = coreprovider.RouteOrderUpdate
	msg.Parsed = &coreprovider.OrderEvent{Symbol: symbol, OrderID: id, Status: status, FilledQty: filled, AvgPrice: avg, Time: parseTime(fmt.Sprint(env["time"]))}
	return nil
}

func (w *WSRouter) parseBalance(msg *coreprovider.RoutedMessage, event string, env map[string]any) error {
	accounts, _ := env["accounts"].([]any)
	balances := make([]core.Balance, 0, len(accounts))
	for _, acct := range accounts {
		row, _ := acct.(map[string]any)
		asset := strings.ToUpper(fmt.Sprint(row["currency"]))
		free := parseDecimal(fmt.Sprint(row["balance"]))
		balances = append(balances, core.Balance{Asset: asset, Available: free, Time: parseTime(fmt.Sprint(env["time"]))})
	}
	msg.Topic = topicFromProviderName(event, "")
	msg.Route = coreprovider.RouteBalanceSnapshot
	msg.Parsed = &coreprovider.BalanceEvent{Balances: balances}
	return nil
}
