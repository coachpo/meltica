package routing

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/coachpo/meltica/core"
	coreprovider "github.com/coachpo/meltica/core/provider"
	corews "github.com/coachpo/meltica/core/ws"
	"github.com/coachpo/meltica/providers/binance/provider/status"
)

func (w *WSRouter) parsePrivateMessage(msg *RoutedMessage, payload []byte) error {
	var env struct {
		Event string `json:"e"`
	}
	if err := json.Unmarshal(payload, &env); err != nil {
		return err
	}

	switch env.Event {
	case "ORDER_TRADE_UPDATE":
		return w.parseOrderUpdate(msg, payload)
	case "outboundAccountPosition", "balanceUpdate":
		return w.parseBalanceUpdate(msg, payload, env.Event)
	default:
		return nil
	}
}

func (w *WSRouter) parseOrderUpdate(msg *RoutedMessage, payload []byte) error {
	var ou struct {
		EventTime int64 `json:"E"`
		Order     struct {
			Symbol    string `json:"s"`
			ID        int64  `json:"i"`
			Status    string `json:"X"`
			FilledQty string `json:"z"`
			AvgPrice  string `json:"ap"`
			EventTime int64  `json:"T"`
		} `json:"o"`
	}
	if err := json.Unmarshal(payload, &ou); err != nil {
		return err
	}

	filled, _ := parseDecimalToRat(ou.Order.FilledQty)
	avg, _ := parseDecimalToRat(ou.Order.AvgPrice)
	msg.Route = coreprovider.RouteOrderUpdate
	msg.Topic = topicFromChannel(BNXOrderChannel, ou.Order.Symbol)
	msg.Parsed = &coreprovider.OrderEvent{
		Symbol:    ou.Order.Symbol,
		OrderID:   fmt.Sprintf("%d", ou.Order.ID),
		Status:    status.MapOrderStatus(ou.Order.Status),
		FilledQty: filled,
		AvgPrice:  avg,
		Time:      time.UnixMilli(ou.Order.EventTime),
	}
	return nil
}

func (w *WSRouter) parseBalanceUpdate(msg *RoutedMessage, payload []byte, event string) error {
	msg.Route = coreprovider.RouteBalanceSnapshot

	switch event {
	case "outboundAccountPosition":
		var oap struct {
			EventTime int64 `json:"E"`
			Balances  []struct {
				Asset string `json:"a"`
				Free  string `json:"f"`
			} `json:"B"`
		}
		if err := json.Unmarshal(payload, &oap); err != nil {
			return err
		}
		var be coreprovider.BalanceEvent
		for _, b := range oap.Balances {
			amt, _ := parseDecimalToRat(b.Free)
			be.Balances = append(be.Balances, core.Balance{Asset: b.Asset, Available: amt, Time: time.UnixMilli(oap.EventTime)})
		}
		msg.Topic = corews.UserBalanceTopic()
		msg.Route = coreprovider.RouteBalanceSnapshot
		msg.Parsed = &be
		return nil
	case "balanceUpdate":
		var bu struct {
			EventTime int64  `json:"E"`
			Asset     string `json:"a"`
			Delta     string `json:"d"`
		}
		if err := json.Unmarshal(payload, &bu); err != nil {
			return err
		}
		amt, _ := parseDecimalToRat(bu.Delta)
		msg.Topic = corews.UserBalanceTopic()
		msg.Route = coreprovider.RouteBalanceSnapshot
		msg.Parsed = &coreprovider.BalanceEvent{
			Balances: []core.Balance{
				{Asset: bu.Asset, Available: amt, Time: time.UnixMilli(bu.EventTime)},
			},
		}
		return nil
	default:
		return nil
	}
}
