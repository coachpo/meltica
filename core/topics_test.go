package core

import (
	"strings"
	"testing"
)

type stubTopicTranslator struct {
	native map[Topic]string
}

func (s stubTopicTranslator) Native(topic Topic) (string, error) {
	if val, ok := s.native[topic]; ok {
		return val, nil
	}
	return "", ErrNotSupported
}

func (s stubTopicTranslator) Canonical(native string) (Topic, error) {
	for topic, val := range s.native {
		if val == native {
			return topic, nil
		}
	}
	return "", ErrNotSupported
}

func TestCanonicalTopicRoundTrip(t *testing.T) {
	name := ExchangeName("stub")
	RegisterTopicTranslator(name, stubTopicTranslator{native: map[Topic]string{
		TopicTrade:       "trade",
		TopicTicker:      "ticker",
		TopicBookDelta:   "depth",
		TopicUserOrder:   "order",
		TopicUserBalance: "balance",
	}})

	cases := []struct {
		topic   Topic
		symbol  string
		want    string
		wantErr bool
	}{
		{TopicTrade, "btc-usdt", "mkt.trade:BTC-USDT", false},
		{TopicTicker, "ETH-usdt", "mkt.ticker:ETH-USDT", false},
		{TopicBookDelta, "bnb-usdt", "mkt.book.delta:BNB-USDT", false},
		{TopicUserOrder, "ada-usdt", "user.order:ADA-USDT", false},
		{TopicUserBalance, "", "user.balance", false},
		{TopicUserBalance, "ignored", "user.balance:IGNORED", false},
		{TopicTrade, "", "", true},
	}

	for _, tc := range cases {
		got, err := CanonicalTopic(tc.topic, tc.symbol)
		if tc.wantErr {
			if err == nil {
				t.Fatalf("expected error for %v %q", tc.topic, tc.symbol)
			}
			continue
		}
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != tc.want {
			t.Fatalf("canonical topic mismatch: got %q want %q", got, tc.want)
		}

		parsedTopic, parsedSymbol, err := ParseTopic(got)
		if err != nil {
			t.Fatalf("unexpected parse error: %v", err)
		}
		if parsedTopic != Topic(normalizeTopic(tc.topic)) {
			t.Fatalf("parsed topic mismatch: got %q want %q", parsedTopic, normalizeTopic(tc.topic))
		}
		if TopicRequiresSymbol(tc.topic) {
			wantSymbol := strings.ToUpper(tc.symbol)
			if parsedSymbol != wantSymbol {
				t.Fatalf("parsed symbol mismatch: got %q want %q", parsedSymbol, wantSymbol)
			}
		}

		native, err := NativeTopic(name, tc.topic)
		if err != nil {
			t.Fatalf("native lookup error: %v", err)
		}
		if native == "" {
			t.Fatalf("native should not be empty")
		}

		back, err := CanonicalTopicFromNative(name, native)
		if err != nil {
			t.Fatalf("canonical lookup error: %v", err)
		}
		if back != tc.topic {
			t.Fatalf("canonical topic mismatch: got %q want %q", back, tc.topic)
		}
	}
}
