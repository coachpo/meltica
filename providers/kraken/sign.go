package kraken

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"net/http"
	"net/url"
	"strconv"

	"github.com/yourorg/meltica/transport"
)

// newSigner returns a transport.Signer compatible with Kraken's HMAC-SHA512 requirements.
func newSigner(key, secret string) transport.Signer {
	secretBytes, err := base64.StdEncoding.DecodeString(secret)
	if err != nil {
		secretBytes = []byte(secret)
	}
	return func(method, path string, q map[string]string, body []byte, ts int64) (http.Header, error) {
		h := http.Header{}
		h.Set("API-Key", key)
		if method != http.MethodPost {
			return h, nil
		}
		post := string(body)
		if post == "" {
			return h, nil
		}
		vals, err := url.ParseQuery(post)
		if err != nil {
			return h, err
		}
		nonce := vals.Get("nonce")
		if nonce == "" {
			nonce = strconv.FormatInt(ts, 10)
			vals.Set("nonce", nonce)
			post = vals.Encode()
		}
		sum := sha256.Sum256([]byte(nonce + post))
		mac := hmac.New(sha512.New, secretBytes)
		mac.Write(append([]byte(path), sum[:]...))
		h.Set("API-Sign", base64.StdEncoding.EncodeToString(mac.Sum(nil)))
		return h, nil
	}
}
