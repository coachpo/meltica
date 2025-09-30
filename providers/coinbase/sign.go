package coinbase

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/yourorg/meltica/transport"
)

func newSigner(apiKey, secret, passphrase string) (transport.Signer, error) {
	secretBytes, err := base64.StdEncoding.DecodeString(secret)
	if err != nil {
		return nil, err
	}
	return func(method, path string, q map[string]string, body []byte, ts int64) (http.Header, error) {
		nonce := strconv.FormatInt(ts/1000, 10)
		reqPath := path
		if len(q) > 0 {
			reqPath = reqPath + "?" + encodeQuery(q)
		}
		message := nonce + strings.ToUpper(method) + reqPath
		if len(body) > 0 {
			message += string(body)
		}
		mac := hmac.New(sha256.New, secretBytes)
		mac.Write([]byte(message))
		sig := base64.StdEncoding.EncodeToString(mac.Sum(nil))
		hdr := http.Header{}
		hdr.Set("CB-ACCESS-KEY", apiKey)
		hdr.Set("CB-ACCESS-SIGN", sig)
		hdr.Set("CB-ACCESS-TIMESTAMP", nonce)
		hdr.Set("CB-ACCESS-PASSPHRASE", passphrase)
		return hdr, nil
	}, nil
}

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

func encodeQuery(q map[string]string) string {
	if len(q) == 0 {
		return ""
	}
	params := url.Values{}
	keys := make([]string, 0, len(q))
	for k := range q {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		params.Add(k, q[k])
	}
	return params.Encode()
}
