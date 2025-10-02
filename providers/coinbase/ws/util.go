package ws

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/coachpo/meltica/core"
)

func buildWSSignature(apiKey, secret, passphrase string) (map[string]any, error) {
	secretBytes, err := base64.StdEncoding.DecodeString(secret)
	if err != nil {
		return nil, err
	}
	nonce := strconv.FormatInt(time.Now().Unix(), 10)
	message := nonce + "GET" + "/users/self/verify"
	mac := hmac.New(sha256.New, secretBytes)
	mac.Write([]byte(message))
	sig := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	return map[string]any{
		"signature":  sig,
		"key":        apiKey,
		"passphrase": passphrase,
		"timestamp":  nonce,
	}, nil
}

func parseDecimal(v string) *big.Rat {
	if v == "" {
		return nil
	}
	var r big.Rat
	if _, ok := r.SetString(v); !ok {
		return nil
	}
	return &r
}

func parseTime(val string) time.Time {
	if val == "" {
		return time.Time{}
	}
	if t, err := time.Parse(time.RFC3339Nano, val); err == nil {
		return t.UTC()
	}
	if t, err := time.Parse(time.RFC3339, val); err == nil {
		return t.UTC()
	}
	return time.Time{}
}

func mapStatus(status, reason string) core.OrderStatus {
	switch strings.ToLower(status) {
	case "received", "open", "pending", "active":
		return core.OrderNew
	case "done", "settled":
		switch strings.ToLower(reason) {
		case "canceled":
			return core.OrderCanceled
		case "rejected":
			return core.OrderRejected
		default:
			return core.OrderFilled
		}
	default:
		return core.OrderNew
	}
}
