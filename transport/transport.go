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
	"strconv"
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

func (c *Client) Do(ctx context.Context, method, path string, query map[string]string, body []byte, signed bool, out any) error {
	_, err := c.DoWithHeaders(ctx, method, path, query, body, signed, out)
	return err
}

func (c *Client) DoWithHeaders(ctx context.Context, method, path string, query map[string]string, body []byte, signed bool, out any) (http.Header, error) {
	if c.HTTP == nil {
		return nil, errors.New("http client not configured")
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	base, err := url.Parse(c.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base url: %w", err)
	}
	u := base.ResolveReference(&url.URL{Path: base.Path + path})
	var signedHeaders http.Header
	if signed && c.Signer != nil {
		ts := time.Now().UnixMilli()
		hdrs, err := c.Signer(method, path, query, body, ts)
		if err != nil {
			return nil, err
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
		return nil, err
	}
	for k, v := range c.DefaultHeaders {
		if v == "" {
			continue
		}
		req.Header.Set(k, v)
	}
	if reqBody != nil && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, vs := range signedHeaders {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	if c.RateLimiter != nil {
		if err := c.RateLimiter.Wait(ctx); err != nil {
			return nil, err
		}
		if c.OnRLWait != nil {
			c.OnRLWait(0)
		}
	}
	retries := 0
	for {
		if c.OnAttempt != nil {
			c.OnAttempt(method, u.String(), retries)
		}
		start := time.Now()
		resp, err := c.HTTP.Do(req)
		if err != nil {
			if retries < c.Retry.MaxRetries {
				delay := c.Retry.BaseDelay * (1 << retries)
				if delay > c.Retry.MaxDelay && c.Retry.MaxDelay > 0 {
					delay = c.Retry.MaxDelay
				}
				select {
				case <-ctx.Done():
					if c.OnResult != nil {
						c.OnResult(method, u.String(), retries, 0, ctx.Err(), time.Since(start))
					}
					return nil, ctx.Err()
				case <-time.After(delay):
				}
				retries++
				continue
			}
			if c.OnResult != nil {
				c.OnResult(method, u.String(), retries, 0, err, time.Since(start))
			}
			return nil, err
		}
		hdr := resp.Header.Clone()
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			if retries < c.Retry.MaxRetries {
				if ra := resp.Header.Get("Retry-After"); ra != "" {
					if secs, e := strconv.Atoi(ra); e == nil {
						wait := time.Duration(secs) * time.Second
						if c.OnRLWait != nil {
							c.OnRLWait(wait)
						}
						resp.Body.Close()
						time.Sleep(wait)
					} else {
						resp.Body.Close()
					}
				} else {
					delay := c.Retry.BaseDelay * (1 << retries)
					if delay > c.Retry.MaxDelay && c.Retry.MaxDelay > 0 {
						delay = c.Retry.MaxDelay
					}
					if c.OnRLWait != nil {
						c.OnRLWait(delay)
					}
					resp.Body.Close()
					time.Sleep(delay)
				}
				retries++
				continue
			}
			data, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			var httpErr error
			if c.OnHTTPError != nil {
				httpErr = c.OnHTTPError(resp.StatusCode, data)
			} else {
				httpErr = fmt.Errorf("http %d: %s", resp.StatusCode, string(data))
			}
			if c.OnResult != nil {
				c.OnResult(method, u.String(), retries, resp.StatusCode, httpErr, time.Since(start))
			}
			return nil, httpErr
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			data, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			var httpErr error
			if c.OnHTTPError != nil {
				httpErr = c.OnHTTPError(resp.StatusCode, data)
			} else {
				httpErr = fmt.Errorf("http %d: %s", resp.StatusCode, string(data))
			}
			if c.OnResult != nil {
				c.OnResult(method, u.String(), retries, resp.StatusCode, httpErr, time.Since(start))
			}
			return nil, httpErr
		}
		if out == nil {
			resp.Body.Close()
			if c.OnResult != nil {
				c.OnResult(method, u.String(), retries, resp.StatusCode, nil, time.Since(start))
			}
			return hdr, nil
		}
		dec := json.NewDecoder(resp.Body)
		decodeErr := dec.Decode(out)
		resp.Body.Close()
		if c.OnResult != nil {
			c.OnResult(method, u.String(), retries, resp.StatusCode, decodeErr, time.Since(start))
		}
		return hdr, decodeErr
	}
}
