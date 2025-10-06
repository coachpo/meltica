package topics

import (
	"testing"
)

func TestParseValidTopics(t *testing.T) {
	cases := []struct {
		name    string
		topic   string
		channel string
		symbol  string
	}{
		{"trade", Trade("BTC-USDT"), TopicTrade, "BTC-USDT"},
		{"ticker", Ticker("ETH-USDT"), TopicTicker, "ETH-USDT"},
		{"book", Book("SOL-USDT"), TopicBook, "SOL-USDT"},
		{"user order", UserOrder("BNB-USDT"), TopicUserOrder, "BNB-USDT"},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			channel, symbol, err := Parse(tt.topic)
			if err != nil {
				t.Fatalf("Parse(%q) returned error: %v", tt.topic, err)
			}
			if channel != tt.channel {
				t.Fatalf("expected channel %q, got %q", tt.channel, channel)
			}
			if symbol != tt.symbol {
				t.Fatalf("expected symbol %q, got %q", tt.symbol, symbol)
			}
		})
	}
}

func TestParseInvalidTopics(t *testing.T) {
	invalid := []string{"", "trade", "trade:", ":BTC-USDT", " trade : "}
	for _, topic := range invalid {
		if _, _, err := Parse(topic); err == nil {
			t.Fatalf("expected error for topic %q", topic)
		}
	}
}

func FuzzParse(f *testing.F) {
	for _, seed := range []string{
		Trade("BTC-USDT"),
		Ticker("ETH-USDT"),
		Book("SOL-USDT"),
		"invalid",
		"another-invalid",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, topic string) {
		channel, symbol, err := Parse(topic)
		if err == nil {
			if channel == "" {
				t.Fatalf("channel empty for topic %q", topic)
			}
			if symbol == "" {
				t.Fatalf("symbol empty for topic %q", topic)
			}
		}
	})
}
