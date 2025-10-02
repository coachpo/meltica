package binance

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/coachpo/meltica/core"
	"github.com/gorilla/websocket"
)

const (
	publicStreamURL    = "wss://stream.binance.com:9443/stream"
	wsHandshakeTimeout = 10 * time.Second
)

type wsSub struct {
	c    chan core.Message
	err  chan error
	conn *websocket.Conn
}

func (s *wsSub) C() <-chan core.Message { return s.c }
func (s *wsSub) Err() <-chan error      { return s.err }

func (s *wsSub) Close() error {
	if s.conn != nil {
		return s.conn.Close()
	}
	return nil
}

func newWSSub(conn *websocket.Conn) *wsSub {
	return &wsSub{
		c:    make(chan core.Message, 1024),
		err:  make(chan error, 1),
		conn: conn,
	}
}

func (w ws) SubscribePublic(ctx context.Context, topics ...string) (core.Subscription, error) {
	if len(topics) == 0 {
		return nil, fmt.Errorf("no topics provided")
	}
	streams := w.buildStreams(topics)
	if len(streams) == 0 {
		return nil, fmt.Errorf("no streams derived from topics")
	}
	q := url.Values{}
	q.Set("streams", strings.Join(streams, "/"))
	endpoint := publicStreamURL + "?" + q.Encode()
	dialer := websocket.Dialer{HandshakeTimeout: wsHandshakeTimeout}
	conn, _, err := dialer.DialContext(ctx, endpoint, nil)
	if err != nil {
		return nil, err
	}
	sub := newWSSub(conn)
	go w.readLoop(sub)
	return sub, nil
}

func (w ws) buildStreams(topics []string) []string {
	streams := make([]string, 0, len(topics))
	for _, topic := range topics {
		channel, instrument := splitTopic(topic)
		providerChannel := mapper.ToProviderChannel(channel)
		if instrument == "" {
			streams = append(streams, topic)
			continue
		}
		if providerChannel == "" {
			providerChannel = strings.ToLower(channel)
		}
		binanceSymbol := strings.ToLower(core.CanonicalToBinance(instrument))
		streams = append(streams, binanceSymbol+"@"+providerChannel)
	}
	return streams
}

func (w ws) readLoop(sub *wsSub) {
	defer close(sub.c)
	defer close(sub.err)
	for {
		_, data, err := sub.conn.ReadMessage()
		if err != nil {
			sub.err <- err
			return
		}
		msg := core.Message{Raw: data, At: time.Now()}
		if err := w.parsePublicMessage(&msg, data); err != nil {
			sub.err <- err
			return
		}
		sub.c <- msg
	}
}

// Private subscription implemented in ws_private.go
