package binance

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/url"
	"strconv"
)

func signHMAC(secret string, method, path string, q map[string]string, body []byte, ts int64) (http.Header, error) {
	if q == nil {
		q = map[string]string{}
	}
	q["timestamp"] = strconv.FormatInt(ts, 10)
	uv := url.Values{}
	for k, v := range q {
		uv.Set(k, v)
	}
	payload := uv.Encode()
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(payload))
	sig := hex.EncodeToString(h.Sum(nil))
	q["signature"] = sig
	hdr := http.Header{}
	return hdr, nil
}
