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

func withTopicTranslators(t *testing.T, fn func()) {
	t.Helper()
	topicTranslatorMu.Lock()
	original := make(map[ExchangeName]TopicTranslator, len(topicTranslators))
	for k, v := range topicTranslators {
		original[k] = v
	}
	topicTranslators = make(map[ExchangeName]TopicTranslator)
	topicTranslatorMu.Unlock()

	t.Cleanup(func() {
		topicTranslatorMu.Lock()
		topicTranslators = make(map[ExchangeName]TopicTranslator, len(original))
		for k, v := range original {
			topicTranslators[k] = v
		}
		topicTranslatorMu.Unlock()
	})

	fn()
}

func TestCanonicalTopicRoundTrip(t *testing.T) {
	withTopicTranslators(t, func() {
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
	})
}

func TestTopicTranslatorErrorsAndMustCanonical(t *testing.T) {
	withTopicTranslators(t, func() {
		if _, err := TopicTranslatorFor("missing"); err == nil {
			t.Fatal("expected error for missing translator")
		}

		RegisterTopicTranslator("binance", stubTopicTranslator{native: map[Topic]string{TopicTrade: "trade"}})
		if _, err := NativeTopic("binance", TopicBookDelta); err == nil {
			t.Fatal("expected error for unsupported topic")
		}

		func() {
			defer func() {
				if r := recover(); r == nil {
					t.Fatal("expected panic from MustCanonicalTopic")
				}
			}()
			_ = MustCanonicalTopic(TopicTrade, "")
		}()

		if TopicRequiresSymbol(TopicUserBalance) {
			t.Fatal("balance topic should not require symbol")
		}
		if !TopicRequiresSymbol(TopicTrade) {
			t.Fatal("trade topic should require symbol")
		}
	})
}

func TestParseTopicValidation(t *testing.T) {
	if _, _, err := ParseTopic(" "); err == nil {
		t.Fatal("expected error for blank input")
	}
	if _, _, err := ParseTopic("bogus:spot"); err == nil {
		t.Fatal("expected error for unknown topic")
	}
	if _, _, err := ParseTopic("mkt.trade"); err == nil {
		t.Fatal("expected error when symbol missing for trade topic")
	}
}
