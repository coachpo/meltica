package okx

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// okxSigner builds OKX auth headers for private requests.
func okxSigner(apiKey, secret, passphrase string) func(method, path string, q map[string]string, body []byte, ts int64) (http.Header, error) {
	return func(method, path string, q map[string]string, body []byte, ts int64) (http.Header, error) {
		// ts in seconds per OKX
		tsStr := fmt.Sprintf("%d", ts/1000)
		prehash := tsStr + strings.ToUpper(method) + path
		if len(q) > 0 {
			// OKX signs path+query
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
