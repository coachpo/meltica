package connection

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

func TestFrameReassemblyLargePayload(t *testing.T) {
	upgrader := websocket.Upgrader{
		ReadBufferSize:  2048,
		WriteBufferSize: 2048,
		CheckOrigin:     func(*http.Request) bool { return true },
	}

	largePayload := strings.Repeat("a", 128*1024)
	message := fmt.Sprintf(`{"type":"test","payload":"%s"}`, largePayload)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		require.NoError(t, err)
		defer conn.Close()

		writer, err := conn.NextWriter(websocket.TextMessage)
		require.NoError(t, err)
		chunk := []byte(message)
		const step = 2048
		for i := 0; i < len(chunk); i += step {
			end := i + step
			if end > len(chunk) {
				end = len(chunk)
			}
			_, err = writer.Write(chunk[i:end])
			require.NoError(t, err)
		}
		require.NoError(t, writer.Close())
		time.Sleep(50 * time.Millisecond)
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer conn.Close()

	msgType, data, err := conn.ReadMessage()
	require.NoError(t, err)
	require.Equal(t, websocket.TextMessage, msgType)
	require.Greater(t, len(data), 120*1024)

	var parsed struct {
		Type    string `json:"type"`
		Payload string `json:"payload"`
	}
	require.NoError(t, json.Unmarshal(data, &parsed))
	require.Equal(t, "test", parsed.Type)
	require.Equal(t, len(largePayload), len(parsed.Payload))
	require.True(t, strings.HasPrefix(parsed.Payload, "aaaa"))
}
