package filter

import (
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"time"

	corestreams "github.com/coachpo/meltica/core/streams"
	mdfilter "github.com/coachpo/meltica/marketdata/filter"
)

// PrivateMessageParser parses Binance private WebSocket messages
type PrivateMessageParser struct{}

// NewPrivateMessageParser creates a new parser for Binance private messages
func NewPrivateMessageParser() *PrivateMessageParser {
	return &PrivateMessageParser{}
}

// ParseAccountUpdate parses account update messages from Binance
func (p *PrivateMessageParser) ParseAccountUpdate(raw []byte) (*mdfilter.AccountEvent, error) {
	var msg struct {
		EventType string `json:"e"`
		EventTime int64  `json:"E"`
		Balances  []struct {
			Asset  string `json:"a"`
			Free   string `json:"f"`
			Locked string `json:"l"`
		} `json:"B"`
	}

	if err := json.Unmarshal(raw, &msg); err != nil {
		return nil, fmt.Errorf("failed to parse account update: %w", err)
	}

	if msg.EventType != "outboundAccountPosition" {
		return nil, fmt.Errorf("unexpected event type: %s", msg.EventType)
	}

	// For now, we'll create a summary event. In practice, you might want to
	// create separate events per asset or handle this differently
	if len(msg.Balances) == 0 {
		return nil, nil
	}

	// Create a summary event for the first balance (simplified)
	balance := msg.Balances[0]
	free, ok := new(big.Rat).SetString(balance.Free)
	if !ok {
		return nil, fmt.Errorf("invalid free balance: %s", balance.Free)
	}

	locked, ok := new(big.Rat).SetString(balance.Locked)
	if !ok {
		return nil, fmt.Errorf("invalid locked balance: %s", balance.Locked)
	}

	total := new(big.Rat).Add(free, locked)

	return &mdfilter.AccountEvent{
		Symbol:    balance.Asset,
		Balance:   total,
		Available: free,
		Locked:    locked,
	}, nil
}

// ParseOrderUpdate parses order update messages from Binance
func (p *PrivateMessageParser) ParseOrderUpdate(raw []byte) (*mdfilter.OrderEvent, error) {
	var msg struct {
		EventType         string `json:"e"`
		EventTime         int64  `json:"E"`
		Symbol            string `json:"s"`
		ClientOrderID     string `json:"c"`
		Side              string `json:"S"`
		OrderType         string `json:"o"`
		TimeInForce       string `json:"f"`
		Quantity          string `json:"q"`
		Price             string `json:"p"`
		StopPrice         string `json:"P"`
		IcebergQuantity   string `json:"F"`
		OrderListID       int64  `json:"g"`
		OrigClientOrderID string `json:"C"`
		ExecutionType     string `json:"x"`
		OrderStatus       string `json:"X"`
		RejectReason      string `json:"r"`
		OrderID           int64  `json:"i"`
		LastExecutedQty   string `json:"l"`
		CumulativeQty     string `json:"z"`
		LastExecutedPrice string `json:"L"`
		Commission        string `json:"n"`
		CommissionAsset   string `json:"N"`
		TransactionTime   int64  `json:"T"`
		TradeID           int64  `json:"t"`
		IsWorking         bool   `json:"w"`
		IsMaker           bool   `json:"m"`
		Ignore            int64  `json:"I"`
	}

	if err := json.Unmarshal(raw, &msg); err != nil {
		return nil, fmt.Errorf("failed to parse order update: %w", err)
	}

	if msg.EventType != "executionReport" {
		return nil, fmt.Errorf("unexpected event type: %s", msg.EventType)
	}

	// Parse quantities
	quantity, ok := new(big.Rat).SetString(msg.Quantity)
	if !ok {
		return nil, fmt.Errorf("invalid quantity: %s", msg.Quantity)
	}

	var price *big.Rat
	if msg.Price != "" && msg.Price != "0" {
		price, ok = new(big.Rat).SetString(msg.Price)
		if !ok {
			return nil, fmt.Errorf("invalid price: %s", msg.Price)
		}
	}

	// Normalize order status
	status := p.normalizeOrderStatus(msg.OrderStatus)

	return &mdfilter.OrderEvent{
		Symbol:       msg.Symbol,
		OrderID:      fmt.Sprintf("%d", msg.OrderID),
		Side:         msg.Side,
		Price:        price,
		Quantity:     quantity,
		Status:       status,
		Type:         msg.OrderType,
		TimeInForce:  msg.TimeInForce,
	}, nil
}

// normalizeOrderStatus converts Binance order status to normalized status
func (p *PrivateMessageParser) normalizeOrderStatus(binanceStatus string) string {
	switch binanceStatus {
	case "NEW":
		return "new"
	case "PARTIALLY_FILLED":
		return "partially_filled"
	case "FILLED":
		return "filled"
	case "CANCELED":
		return "cancelled"
	case "PENDING_CANCEL":
		return "pending_cancel"
	case "REJECTED":
		return "rejected"
	case "EXPIRED":
		return "expired"
	default:
		return strings.ToLower(binanceStatus)
	}
}

// ParseBalanceUpdate parses balance update messages
func (p *PrivateMessageParser) ParseBalanceUpdate(raw []byte) (*mdfilter.AccountEvent, error) {
	var msg struct {
		EventType string `json:"e"`
		EventTime int64  `json:"E"`
		Asset     string `json:"a"`
		Balance   string `json:"b"`
	}

	if err := json.Unmarshal(raw, &msg); err != nil {
		return nil, fmt.Errorf("failed to parse balance update: %w", err)
	}

	if msg.EventType != "balanceUpdate" {
		return nil, fmt.Errorf("unexpected event type: %s", msg.EventType)
	}

	balance, ok := new(big.Rat).SetString(msg.Balance)
	if !ok {
		return nil, fmt.Errorf("invalid balance: %s", msg.Balance)
	}

	return &mdfilter.AccountEvent{
		Symbol:    msg.Asset,
		Balance:   balance,
		Available: balance,
		Locked:    new(big.Rat),
	}, nil
}

// ParseMessage parses any Binance private message and returns the appropriate event envelope
func (p *PrivateMessageParser) ParseMessage(routedMsg corestreams.RoutedMessage) (mdfilter.EventEnvelope, error) {
	var baseMsg struct {
		EventType string `json:"e"`
	}

	if err := json.Unmarshal(routedMsg.Raw, &baseMsg); err != nil {
		return mdfilter.EventEnvelope{}, fmt.Errorf("failed to parse message type: %w", err)
	}

	envelope := mdfilter.EventEnvelope{
		Channel:   mdfilter.ChannelPrivateWS,
		Timestamp: routedMsg.At,
	}

	switch baseMsg.EventType {
	case "outboundAccountPosition":
		accountEvent, err := p.ParseAccountUpdate(routedMsg.Raw)
		if err != nil {
			return mdfilter.EventEnvelope{}, fmt.Errorf("failed to parse account update: %w", err)
		}
		envelope.Kind = mdfilter.EventKindAccount
		envelope.AccountEvent = accountEvent
		if accountEvent != nil {
			envelope.Symbol = accountEvent.Symbol
		}

	case "executionReport":
		orderEvent, err := p.ParseOrderUpdate(routedMsg.Raw)
		if err != nil {
			return mdfilter.EventEnvelope{}, fmt.Errorf("failed to parse order update: %w", err)
		}
		envelope.Kind = mdfilter.EventKindOrder
		envelope.OrderEvent = orderEvent
		if orderEvent != nil {
			envelope.Symbol = orderEvent.Symbol
		}

	case "balanceUpdate":
		accountEvent, err := p.ParseBalanceUpdate(routedMsg.Raw)
		if err != nil {
			return mdfilter.EventEnvelope{}, fmt.Errorf("failed to parse balance update: %w", err)
		}
		envelope.Kind = mdfilter.EventKindAccount
		envelope.AccountEvent = accountEvent
		if accountEvent != nil {
			envelope.Symbol = accountEvent.Symbol
		}

	default:
		return mdfilter.EventEnvelope{}, fmt.Errorf("unsupported event type: %s", baseMsg.EventType)
	}

	return envelope, nil
}

// ParseError handles parsing errors and creates appropriate error envelopes
func (p *PrivateMessageParser) ParseError(err error, raw []byte) mdfilter.EventEnvelope {
	return mdfilter.EventEnvelope{
		Kind:      mdfilter.EventKindUnknown,
		Channel:   mdfilter.ChannelPrivateWS,
		Timestamp: time.Now(),
		RestResponse: &mdfilter.RestResponse{
			StatusCode: 500,
			Error:      err,
		},
	}
}