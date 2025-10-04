package routing

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/coachpo/meltica/core"
	coreprovider "github.com/coachpo/meltica/core/provider"
)

type okxPrivateEnvelope struct {
	Arg struct {
		Channel string `json:"channel"`
		InstID  string `json:"instId"`
	} `json:"arg"`
	Data  []json.RawMessage `json:"data"`
	Event string            `json:"event"`
}

func (w *WSRouter) parsePrivateMessage(msg *coreprovider.RoutedMessage, raw []byte) error {
	var env okxPrivateEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil
	}
	if len(env.Data) == 0 {
		msg.Topic = env.Event
		msg.Route = coreprovider.RouteUnknown
		return nil
	}
	switch env.Arg.Channel {
	case OKXTopicOrders:
		return w.parseOrderUpdate(msg, env.Data)
	case "balance_and_position":
		return w.parseBalanceUpdate(msg, env.Data)
	default:
		msg.Topic = protocolTopicFor(env.Arg.Channel)
		msg.Route = coreprovider.RouteUnknown
		return nil
	}
}

func (w *WSRouter) parseOrderUpdate(msg *coreprovider.RoutedMessage, payload []json.RawMessage) error {
	if len(payload) == 0 {
		return nil
	}
	var rec struct {
		OrdID     string `json:"ordId"`
		InstID    string `json:"instId"`
		State     string `json:"state"`
		AccFillSz string `json:"accFillSz"`
		AvgPx     string `json:"avgPx"`
		Ts        string `json:"ts"`
	}
	if err := json.Unmarshal(payload[len(payload)-1], &rec); err != nil {
		return err
	}
	filled, _ := parseDecimalToRat(rec.AccFillSz)
	avg, _ := parseDecimalToRat(rec.AvgPx)
	topicSymbol := rec.InstID
	msg.Topic = topicFromChannel(OKXTopicOrders, topicSymbol)
	msg.Route = coreprovider.RouteOrderUpdate
	msg.Parsed = &coreprovider.OrderEvent{
		Symbol:    topicSymbol,
		OrderID:   rec.OrdID,
		Status:    mapOKXStatus(rec.State),
		FilledQty: filled,
		AvgPrice:  avg,
		Time:      parseMillis(rec.Ts),
	}
	return nil
}

func (w *WSRouter) parseBalanceUpdate(msg *coreprovider.RoutedMessage, payload []json.RawMessage) error {
	balances := make([]core.Balance, 0)
	for _, raw := range payload {
		var entry struct {
			BalData []struct {
				Ccy     string `json:"ccy"`
				CashBal string `json:"cashBal"`
			} `json:"balData"`
		}
		if err := json.Unmarshal(raw, &entry); err != nil {
			continue
		}
		for _, bal := range entry.BalData {
			amt, _ := parseDecimalToRat(bal.CashBal)
			balances = append(balances, core.Balance{Asset: strings.ToUpper(bal.Ccy), Available: amt, Time: time.Now()})
		}
	}
	if len(balances) == 0 {
		return nil
	}
	msg.Topic = topicFromChannel(OKXTopicAccount, "")
	msg.Route = coreprovider.RouteBalanceSnapshot
	msg.Parsed = &coreprovider.BalanceEvent{Balances: balances}
	return nil
}
