package rest

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// newSigner builds OKX auth headers for private requests.
func newSigner(apiKey, secret, passphrase string) func(method, path string, q map[string]string, body []byte, ts int64) (http.Header, error) {
	return func(method, path string, q map[string]string, body []byte, ts int64) (http.Header, error) {
		tsStr := fmt.Sprintf("%d", ts/1000)
		prehash := tsStr + strings.ToUpper(method) + path
		if len(q) > 0 {
			vals := url.Values{}
			for k, v := range q {
				vals.Set(k, v)
			}
			prehash += "?" + vals.Encode()
		}
		if len(body) > 0 {
			prehash += string(body)
		}
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write([]byte(prehash))
		sign := base64.StdEncoding.EncodeToString(mac.Sum(nil))
		h := http.Header{}
		h.Set("OK-ACCESS-KEY", apiKey)
		h.Set("OK-ACCESS-SIGN", sign)
		h.Set("OK-ACCESS-TIMESTAMP", tsStr)
		h.Set("OK-ACCESS-PASSPHRASE", passphrase)
		return h, nil
	}
}

// BuildWSSignature creates the payload required to authenticate websocket connections.
func BuildWSSignature(apiKey, secret, passphrase string) map[string]string {
	ts := time.Now().UTC().Format(time.RFC3339)
	prehash := ts + "GET" + "/users/self/verify"
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(prehash))
	sign := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	return map[string]string{
		"apiKey":     apiKey,
		"passphrase": passphrase,
		"timestamp":  ts,
		"sign":       sign,
	}
}
