package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Signer func(method, path string, q map[string]string, body []byte, ts int64) (http.Header, error)

type RetryPolicy struct {
	MaxRetries int
	BaseDelay  time.Duration
	MaxDelay   time.Duration
}

type RateLimiter interface {
	Wait(ctx context.Context) error
}

type Client struct {
	HTTP           *http.Client
	Retry          RetryPolicy
	RateLimiter    RateLimiter
	Signer         Signer
	BaseURL        string
	DefaultHeaders map[string]string
	OnAttempt      func(method, url string, attempt int)
	OnResult       func(method, url string, attempt int, status int, err error, dur time.Duration)
	OnRLWait       func(wait time.Duration)
	OnHTTPError    func(status int, body []byte) error
}

func (c *Client) DoWithHeaders(ctx context.Context, method, path string, query map[string]string, body []byte, signed bool, header http.Header, out any) (http.Header, int, error) {
	if c.HTTP == nil {
		return nil, 0, errors.New("http client not configured")
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	base, err := url.Parse(c.BaseURL)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid base url: %w", err)
	}
	u := base.ResolveReference(&url.URL{Path: base.Path + path})
	var signedHeaders http.Header
	if signed && c.Signer != nil {
		ts := time.Now().UnixMilli()
		hdrs, err := c.Signer(method, path, query, body, ts)
		if err != nil {
			return nil, 0, err
		}
		signedHeaders = hdrs
	}
	if len(query) > 0 {
		q := u.Query()
		for k, v := range query {
			q.Set(k, v)
		}
		u.RawQuery = q.Encode()
	}
	var reqBody io.Reader
	if len(body) > 0 && method != http.MethodGet {
		reqBody = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), reqBody)
	if err != nil {
		return nil, 0, err
	}
	for k, v := range c.DefaultHeaders {
		if v == "" {
			continue
		}
		req.Header.Set(k, v)
	}
	for k, vs := range header {
		for _, v := range vs {
			if v == "" {
				continue
			}
			req.Header.Add(k, v)
		}
	}
	if reqBody != nil && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if reqBody != nil {
		req.GetBody = func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(body)), nil
		}
		req.Body = io.NopCloser(bytes.NewReader(body))
	}
	for k, vs := range signedHeaders {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	retries := 0
	for {
		if c.RateLimiter != nil {
			if err := c.RateLimiter.Wait(ctx); err != nil {
				if c.OnResult != nil {
					c.OnResult(method, u.String(), retries, 0, err, 0)
				}
				return nil, 0, err
			}
			if c.OnRLWait != nil {
				c.OnRLWait(0)
			}
		}

		if retries > 0 && req.GetBody != nil {
			rc, err := req.GetBody()
			if err != nil {
				return nil, 0, err
			}
			req.Body = rc
		}

		if c.OnAttempt != nil {
			c.OnAttempt(method, u.String(), retries)
		}
		start := time.Now()
		resp, err := c.HTTP.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				if c.OnResult != nil {
					c.OnResult(method, u.String(), retries, 0, ctx.Err(), time.Since(start))
				}
				return nil, 0, ctx.Err()
			}
			if retries >= c.Retry.MaxRetries || !isRetryableNetworkError(err) {
				if c.OnResult != nil {
					c.OnResult(method, u.String(), retries, 0, err, time.Since(start))
				}
				return nil, 0, err
			}
			delay := nextBackoff(c.Retry, retries)
			if c.OnRLWait != nil {
				c.OnRLWait(delay)
			}
			if waitErr := waitWithContext(ctx, delay); waitErr != nil {
				if c.OnResult != nil {
					c.OnResult(method, u.String(), retries, 0, waitErr, time.Since(start))
				}
				return nil, 0, waitErr
			}
			retries++
			continue
		}
		status := resp.StatusCode
		hdr := resp.Header.Clone()
		if shouldRetryStatus(status) && retries < c.Retry.MaxRetries {
			wait, ok := parseRetryAfter(resp.Header, time.Now())
			if !ok {
				wait = nextBackoff(c.Retry, retries)
			}
			if c.OnRLWait != nil {
				c.OnRLWait(wait)
			}
			resp.Body.Close()
			if err := waitWithContext(ctx, wait); err != nil {
				if c.OnResult != nil {
					c.OnResult(method, u.String(), retries, status, err, time.Since(start))
				}
				return nil, status, err
			}
			retries++
			continue
		}
		if status < 200 || status >= 300 {
			data, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			var httpErr error
			if c.OnHTTPError != nil {
				httpErr = c.OnHTTPError(status, data)
			} else {
				httpErr = fmt.Errorf("http %d: %s", status, string(data))
			}
			if c.OnResult != nil {
				c.OnResult(method, u.String(), retries, status, httpErr, time.Since(start))
			}
			return nil, status, httpErr
		}
		if out == nil {
			resp.Body.Close()
			if c.OnResult != nil {
				c.OnResult(method, u.String(), retries, status, nil, time.Since(start))
			}
			return hdr, status, nil
		}
		dec := json.NewDecoder(resp.Body)
		decodeErr := dec.Decode(out)
		resp.Body.Close()
		if c.OnResult != nil {
			c.OnResult(method, u.String(), retries, status, decodeErr, time.Since(start))
		}
		return hdr, status, decodeErr
	}
}
