package routing

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/coachpo/meltica/core"
	corestreams "github.com/coachpo/meltica/core/streams"
	coretopics "github.com/coachpo/meltica/core/topics"
	"github.com/coachpo/meltica/exchanges/binance/internal"
	"github.com/coachpo/meltica/exchanges/shared/infra/numeric"
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
			Symbol    string      `json:"s"`
			ID        int64       `json:"i"`
			Status    string      `json:"X"`
			FilledQty json.Number `json:"z"`
			AvgPrice  json.Number `json:"ap"`
			EventTime int64       `json:"T"`
		} `json:"o"`
	}
	if err := json.Unmarshal(payload, &ou); err != nil {
		return err
	}

	filled, _ := numeric.Parse(ou.Order.FilledQty.String())
	avg, _ := numeric.Parse(ou.Order.AvgPrice.String())
	msg.Route = corestreams.RouteOrderUpdate
	msg.Topic = topicFromChannel(BNXOrderChannel, ou.Order.Symbol)
	msg.Parsed = &corestreams.OrderEvent{
		Symbol:    ou.Order.Symbol,
		OrderID:   fmt.Sprintf("%d", ou.Order.ID),
		Status:    internal.MapOrderStatus(ou.Order.Status),
		FilledQty: filled,
		AvgPrice:  avg,
		Time:      time.UnixMilli(ou.Order.EventTime),
	}
	return nil
}

func (w *WSRouter) parseBalanceUpdate(msg *RoutedMessage, payload []byte, event string) error {
	msg.Route = corestreams.RouteBalanceSnapshot

	switch event {
	case "outboundAccountPosition":
		var oap struct {
			EventTime int64 `json:"E"`
			Balances  []struct {
				Asset string      `json:"a"`
				Free  json.Number `json:"f"`
			} `json:"B"`
		}
		if err := json.Unmarshal(payload, &oap); err != nil {
			return err
		}
		var be corestreams.BalanceEvent
		for _, b := range oap.Balances {
			amt, _ := numeric.Parse(b.Free.String())
			be.Balances = append(be.Balances, core.Balance{Asset: b.Asset, Available: amt, Time: time.UnixMilli(oap.EventTime)})
		}
		msg.Topic = coretopics.UserBalance()
		msg.Route = corestreams.RouteBalanceSnapshot
		msg.Parsed = &be
		return nil
	case "balanceUpdate":
		var bu struct {
			EventTime int64       `json:"E"`
			Asset     string      `json:"a"`
			Delta     json.Number `json:"d"`
		}
		if err := json.Unmarshal(payload, &bu); err != nil {
			return err
		}
		amt, _ := numeric.Parse(bu.Delta.String())
		msg.Topic = coretopics.UserBalance()
		msg.Route = corestreams.RouteBalanceSnapshot
		msg.Parsed = &corestreams.BalanceEvent{
			Balances: []core.Balance{
				{Asset: bu.Asset, Available: amt, Time: time.UnixMilli(bu.EventTime)},
			},
		}
		return nil
	default:
		return nil
	}
}
