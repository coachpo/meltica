package ws

import (
	"strings"

	"github.com/gorilla/websocket"

	"github.com/coachpo/meltica/core"
)

func (w *WS) sendSubscriptions(conn *websocket.Conn, topics []string, private bool) error {
	channels := map[string][]string{}
	for _, topic := range topics {
		parts := strings.Split(topic, ":")
		if len(parts) != 2 {
			continue
		}
		channel := parts[0]
		symbol := parts[1]
		native := w.nativeProduct(symbol)
		providerChannel := mapper.ToProviderChannel(channel)
		if providerChannel == "user" && !private {
			continue
		}
		if channel == core.TopicBalance && private {
			channels[providerChannel] = append(channels[providerChannel], "*")
			continue
		}
		if native != "" {
			channels[providerChannel] = append(channels[providerChannel], native)
		}
	}
	payload := map[string]any{
		"type":        "subscribe",
		"product_ids": uniqueFlatten(channels),
		"channels":    buildChannels(channels),
	}
	if private {
		sig, err := buildWSSignature(w.p.APIKey(), w.p.Secret(), w.p.Passphrase())
		if err != nil {
			return err
		}
		for k, v := range sig {
			payload[k] = v
		}
	}
	return conn.WriteJSON(payload)
}

func buildChannels(target map[string][]string) []any {
	if len(target) == 0 {
		return []any{"ticker", "matches", "level2"}
	}
	out := make([]any, 0, len(target))
	for channel, products := range target {
		if len(products) == 0 {
			continue
		}
		out = append(out, map[string]any{
			"name":        channel,
			"product_ids": products,
		})
	}
	return out
}

func uniqueFlatten(ch map[string][]string) []string {
	set := map[string]struct{}{}
	for _, vals := range ch {
		for _, v := range vals {
			if v == "" {
				continue
			}
			set[v] = struct{}{}
		}
	}
	out := make([]string, 0, len(set))
	for v := range set {
		out = append(out, v)
	}
	return out
}
