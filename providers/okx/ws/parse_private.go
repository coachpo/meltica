package ws

import (
	"encoding/json"
	"time"

	"github.com/coachpo/meltica/core"
)

type okxPrivateEnvelope struct {
	Arg struct {
		Channel string `json:"channel"`
		InstID  string `json:"instId"`
	} `json:"arg"`
	Data  []json.RawMessage `json:"data"`
	Event string            `json:"event"`
}

func (w *WS) parsePrivateMessage(msg *core.Message, raw []byte) error {
	var env okxPrivateEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil
	}
	if len(env.Data) == 0 {
		msg.Topic = env.Event
		msg.Event = env.Event
		return nil
	}
	switch env.Arg.Channel {
	case "orders":
		return w.parseOrderUpdate(msg, env.Data)
	case "balance_and_position":
		return w.parseBalanceUpdate(msg, env.Data)
	default:
		msg.Topic = mapper.ToProtocolTopic(env.Arg.Channel)
		return nil
	}
}

func (w *WS) parseOrderUpdate(msg *core.Message, payload []json.RawMessage) error {
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
	msg.Topic = core.OrderTopic(rec.InstID)
	msg.Event = "order"
	msg.Parsed = &core.OrderEvent{
		Symbol:    rec.InstID,
		OrderID:   rec.OrdID,
		Status:    mapOKXStatus(rec.State),
		FilledQty: filled,
		AvgPrice:  avg,
		Time:      parseMillis(rec.Ts),
	}
	return nil
}

func (w *WS) parseBalanceUpdate(msg *core.Message, payload []json.RawMessage) error {
	var balances []core.Balance
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
			balances = append(balances, core.Balance{Asset: bal.Ccy, Available: amt, Time: time.Now()})
		}
	}
	if len(balances) == 0 {
		return nil
	}
	msg.Topic = core.BalanceTopic()
	msg.Event = "balance"
	msg.Parsed = &core.BalanceEvent{Balances: balances}
	return nil
}
