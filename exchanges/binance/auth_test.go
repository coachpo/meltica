package binance

import (
	"context"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"

	routingrest "github.com/coachpo/meltica/exchanges/shared/routing"
)

type listenKeyDispatcher struct {
	messages []routingrest.RESTMessage
	response string
}

func (s *listenKeyDispatcher) Dispatch(_ context.Context, msg routingrest.RESTMessage, out any) error {
	s.messages = append(s.messages, msg)
	if out != nil {
		rv := reflect.ValueOf(out)
		if rv.Kind() == reflect.Ptr && !rv.IsNil() {
			elem := rv.Elem()
			if field := elem.FieldByName("ListenKey"); field.IsValid() && field.CanSet() && field.Kind() == reflect.String {
				field.SetString(s.response)
			}
		}
	}
	return nil
}

func TestListenKeyServiceCreateDispatchesCorrectRequest(t *testing.T) {
	dispatcher := &listenKeyDispatcher{response: "abc123"}
	service := newListenKeyService(dispatcher)
	key, err := service.Create(context.Background())
	require.NoError(t, err)
	require.Equal(t, "abc123", key)
	require.Len(t, dispatcher.messages, 1)
	msg := dispatcher.messages[0]
	require.Equal(t, "POST", msg.Method)
	require.Equal(t, "/api/v3/userDataStream", msg.Path)
	require.False(t, msg.Signed)
}

func TestListenKeyServiceKeepAliveAndCloseDispatch(t *testing.T) {
	dispatcher := &listenKeyDispatcher{}
	service := newListenKeyService(dispatcher)
	require.NoError(t, service.KeepAlive(context.Background(), "listen-key"))
	require.NoError(t, service.Close(context.Background(), "listen-key"))
	require.Len(t, dispatcher.messages, 2)
	keepAlive := dispatcher.messages[0]
	require.Equal(t, "PUT", keepAlive.Method)
	require.Equal(t, "/api/v3/userDataStream", keepAlive.Path)
	require.Equal(t, "listen-key", keepAlive.Query["listenKey"])
	closeMsg := dispatcher.messages[1]
	require.Equal(t, "DELETE", closeMsg.Method)
	require.Equal(t, "listen-key", closeMsg.Query["listenKey"])
}
