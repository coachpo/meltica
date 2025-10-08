package bootstrap

import (
	"math/big"
	"testing"
	"time"
)

func mustRat(t *testing.T, input string) *big.Rat {
	t.Helper()
	r, ok := new(big.Rat).SetString(input)
	if !ok {
		t.Fatalf("invalid rational %q", input)
	}
	return r
}

func requireRatEqual(t *testing.T, got *big.Rat, want string) {
	t.Helper()
	if got == nil {
		t.Fatalf("expected rational %q, got nil", want)
	}
	expected := mustRat(t, want)
	if got.Cmp(expected) != 0 {
		t.Fatalf("expected %s, got %s", expected.RatString(), got.RatString())
	}
}

func TestOrderPrecisionBoundaries(t *testing.T) {
	cases := []struct {
		name         string
		price        string
		stop         string
		quantity     string
		filled       string
		averagePrice string
	}{
		{
			name:         "midPrecision",
			price:        "12345.67890123456789",
			stop:         "10000.00000000000001",
			quantity:     "0.000010000000000001",
			filled:       "0.000009999999999999",
			averagePrice: "12345.67890123456789",
		},
		{
			name:         "tinyValues",
			price:        "0.000000000000123456789",
			stop:         "0.000000000000100000001",
			quantity:     "0.00000000000000123456789",
			filled:       "0.00000000000000100000000",
			averagePrice: "0.000000000000123456789",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			order := Order{
				Price:          mustRat(t, tc.price),
				StopPrice:      mustRat(t, tc.stop),
				Quantity:       mustRat(t, tc.quantity),
				FilledQuantity: mustRat(t, tc.filled),
				AveragePrice:   mustRat(t, tc.averagePrice),
				CreatedAt:      time.Unix(0, 0),
				UpdatedAt:      time.Unix(0, 0),
				VenueTimestamp: time.Unix(0, 0),
			}

			requireRatEqual(t, order.Price, tc.price)
			requireRatEqual(t, order.StopPrice, tc.stop)
			requireRatEqual(t, order.Quantity, tc.quantity)
			requireRatEqual(t, order.FilledQuantity, tc.filled)
			requireRatEqual(t, order.AveragePrice, tc.averagePrice)
		})
	}
}

func TestTradePrecisionBoundaries(t *testing.T) {
	cases := []struct {
		name     string
		price    string
		quantity string
		fee      string
	}{
		{
			name:     "highPrecision",
			price:    "6789.123456789012345",
			quantity: "0.345678901234567",
			fee:      "0.000345678901234",
		},
		{
			name:     "microTrade",
			price:    "0.000012345678901234",
			quantity: "0.000000012345678901",
			fee:      "0.000000000123456789",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			trade := Trade{
				Price:    mustRat(t, tc.price),
				Quantity: mustRat(t, tc.quantity),
				Fee: Fee{
					Asset:  "USDT",
					Amount: mustRat(t, tc.fee),
				},
				ExecutedAt: time.Unix(0, 0),
			}

			requireRatEqual(t, trade.Price, tc.price)
			requireRatEqual(t, trade.Quantity, tc.quantity)
			requireRatEqual(t, trade.Fee.Amount, tc.fee)
		})
	}
}

func TestAccountPrecisionBoundaries(t *testing.T) {
	cases := []struct {
		name             string
		equity           string
		available        string
		used             string
		unrealized       string
		balanceTotal     string
		balanceAvailable string
		balanceHold      string
	}{
		{
			name:             "marginAccount",
			equity:           "123456.789012345678",
			available:        "100000.123456789012",
			used:             "23456.665555556666",
			unrealized:       "-123.000000000001",
			balanceTotal:     "200000.999999999999",
			balanceAvailable: "199000.123456789012",
			balanceHold:      "1000.876543210987",
		},
		{
			name:             "spotAccount",
			equity:           "0.999999999999999",
			available:        "0.555555555555555",
			used:             "0.444444444444444",
			unrealized:       "0.000000000000000",
			balanceTotal:     "1.000000000000000",
			balanceAvailable: "0.600000000000000",
			balanceHold:      "0.400000000000000",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			account := Account{
				Equity:          mustRat(t, tc.equity),
				MarginAvailable: mustRat(t, tc.available),
				MarginUsed:      mustRat(t, tc.used),
				UnrealizedPNL:   mustRat(t, tc.unrealized),
				Balances: []Balance{
					{
						Asset:     "USDT",
						Total:     mustRat(t, tc.balanceTotal),
						Available: mustRat(t, tc.balanceAvailable),
						Hold:      mustRat(t, tc.balanceHold),
					},
				},
				UpdatedAt: time.Unix(0, 0),
			}

			requireRatEqual(t, account.Equity, tc.equity)
			requireRatEqual(t, account.MarginAvailable, tc.available)
			requireRatEqual(t, account.MarginUsed, tc.used)
			requireRatEqual(t, account.UnrealizedPNL, tc.unrealized)
			if len(account.Balances) != 1 {
				t.Fatalf("expected one balance entry, got %d", len(account.Balances))
			}
			requireRatEqual(t, account.Balances[0].Total, tc.balanceTotal)
			requireRatEqual(t, account.Balances[0].Available, tc.balanceAvailable)
			requireRatEqual(t, account.Balances[0].Hold, tc.balanceHold)
		})
	}
}

func TestPositionPrecisionBoundaries(t *testing.T) {
	cases := []struct {
		name        string
		quantity    string
		entryPrice  string
		markPrice   string
		liquidation string
		realized    string
		unrealized  string
		leverage    string
		margin      string
	}{
		{
			name:        "long",
			quantity:    "10.123456789012345",
			entryPrice:  "25000.123456789012",
			markPrice:   "26000.987654321098",
			liquidation: "20000.000000000000",
			realized:    "123.456789012345",
			unrealized:  "10123.456789012345",
			leverage:    "10.000000000000",
			margin:      "1000.123456789012",
		},
		{
			name:        "short",
			quantity:    "-5.000000000000001",
			entryPrice:  "30000.000000000001",
			markPrice:   "29000.000000000001",
			liquidation: "35000.000000000000",
			realized:    "-55.555555555555",
			unrealized:  "5000.000000000000",
			leverage:    "5.500000000000",
			margin:      "900.000000000000",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			position := Position{
				Quantity:         mustRat(t, tc.quantity),
				EntryPrice:       mustRat(t, tc.entryPrice),
				MarkPrice:        mustRat(t, tc.markPrice),
				LiquidationPrice: mustRat(t, tc.liquidation),
				RealizedPNL:      mustRat(t, tc.realized),
				UnrealizedPNL:    mustRat(t, tc.unrealized),
				Leverage:         mustRat(t, tc.leverage),
				Margin:           mustRat(t, tc.margin),
				UpdatedAt:        time.Unix(0, 0),
				VenueTimestamp:   time.Unix(0, 0),
			}

			requireRatEqual(t, position.Quantity, tc.quantity)
			requireRatEqual(t, position.EntryPrice, tc.entryPrice)
			requireRatEqual(t, position.MarkPrice, tc.markPrice)
			requireRatEqual(t, position.LiquidationPrice, tc.liquidation)
			requireRatEqual(t, position.RealizedPNL, tc.realized)
			requireRatEqual(t, position.UnrealizedPNL, tc.unrealized)
			requireRatEqual(t, position.Leverage, tc.leverage)
			requireRatEqual(t, position.Margin, tc.margin)
		})
	}
}
