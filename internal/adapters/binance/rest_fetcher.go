package binance

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	json "github.com/goccy/go-json"
)

const (
	binanceRESTURL        = "https://api.binance.com"
	binanceTestnetRESTURL = "https://testnet.binance.vision"

	// REST API rate limits per Binance documentation
	maxRequestsPerMinute = 1200
	maxOrdersPerSecond   = 10
	maxOrdersPerDay      = 200000
)

// BinanceRESTFetcher implements REST API client with authentication and rate limiting.
type BinanceRESTFetcher struct {
	baseURL     string
	apiKey      string
	secretKey   string
	httpClient  *http.Client
	rateLimiter *RateLimiter
	useTestnet  bool
}

// NewBinanceRESTFetcher creates a production-ready REST client.
func NewBinanceRESTFetcher(apiKey, secretKey string, useTestnet bool) *BinanceRESTFetcher {
	baseURL := binanceRESTURL
	if useTestnet {
		baseURL = binanceTestnetRESTURL
	}

	return &BinanceRESTFetcher{
		baseURL:     baseURL,
		apiKey:      apiKey,
		secretKey:   secretKey,
		httpClient:  &http.Client{Timeout: 10 * time.Second},
		rateLimiter: NewRateLimiter(20), // 20 requests per second = 1200/minute
		useTestnet:  useTestnet,
	}
}

// Fetch retrieves data from Binance REST API endpoint.
func (f *BinanceRESTFetcher) Fetch(ctx context.Context, endpoint string) ([]byte, error) {
	// Parse endpoint to see if it's a full URL or path
	var fullURL string
	if endpoint == "" {
		return nil, fmt.Errorf("endpoint cannot be empty")
	}

	// If endpoint starts with http, use it as-is
	if len(endpoint) > 4 && (endpoint[:4] == "http" || endpoint[:3] == "ws") {
		fullURL = endpoint
	} else {
		// Otherwise, append to base URL
		fullURL = f.baseURL + endpoint
	}

	// Apply rate limiting
	f.rateLimiter.Wait()

	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// Add API key header if available
	if f.apiKey != "" {
		req.Header.Set("X-MBX-APIKEY", f.apiKey)
	}

	// Execute request
	resp, err := f.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http status %d: %s", resp.StatusCode, string(body))
	}

	return body, nil
}

// FetchSigned makes a signed request to Binance (for authenticated endpoints).
func (f *BinanceRESTFetcher) FetchSigned(ctx context.Context, endpoint string, params url.Values) ([]byte, error) {
	if f.secretKey == "" {
		return nil, fmt.Errorf("secret key required for signed requests")
	}

	// Add timestamp
	if params == nil {
		params = url.Values{}
	}
	timestamp := strconv.FormatInt(time.Now().UnixMilli(), 10)
	params.Set("timestamp", timestamp)

	// Generate signature
	queryString := params.Encode()
	signature := f.sign(queryString)
	params.Set("signature", signature)

	// Build full URL
	fullURL := fmt.Sprintf("%s%s?%s", f.baseURL, endpoint, params.Encode())

	// Apply rate limiting
	f.rateLimiter.Wait()

	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// Add API key header
	if f.apiKey != "" {
		req.Header.Set("X-MBX-APIKEY", f.apiKey)
	}

	// Execute request
	resp, err := f.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http status %d: %s", resp.StatusCode, string(body))
	}

	return body, nil
}

// sign generates HMAC SHA256 signature for Binance API.
func (f *BinanceRESTFetcher) sign(payload string) string {
	mac := hmac.New(sha256.New, []byte(f.secretKey))
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

// GetServerTime retrieves Binance server time (useful for synchronization).
func (f *BinanceRESTFetcher) GetServerTime(ctx context.Context) (int64, error) {
	body, err := f.Fetch(ctx, "/api/v3/time")
	if err != nil {
		return 0, err
	}

	type timeResponse struct {
		ServerTime int64 `json:"serverTime"`
	}

	var resp timeResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return 0, fmt.Errorf("parse server time: %w", err)
	}

	return resp.ServerTime, nil
}

// CreateListenKey creates a user data stream listen key.
func (f *BinanceRESTFetcher) CreateListenKey(ctx context.Context) (string, error) {
	if f.apiKey == "" {
		return "", fmt.Errorf("API key required for user data stream")
	}

	// Apply rate limiting
	f.rateLimiter.Wait()

	// Create POST request
	fullURL := f.baseURL + "/api/v3/userDataStream"
	req, err := http.NewRequestWithContext(ctx, "POST", fullURL, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("X-MBX-APIKEY", f.apiKey)

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("http status %d: %s", resp.StatusCode, string(body))
	}

	type listenKeyResponse struct {
		ListenKey string `json:"listenKey"`
	}

	var lkResp listenKeyResponse
	if err := json.Unmarshal(body, &lkResp); err != nil {
		return "", fmt.Errorf("parse listen key: %w", err)
	}

	return lkResp.ListenKey, nil
}

// KeepAliveListenKey extends the validity of a user data stream.
func (f *BinanceRESTFetcher) KeepAliveListenKey(ctx context.Context, listenKey string) error {
	if f.apiKey == "" {
		return fmt.Errorf("API key required")
	}

	// Apply rate limiting
	f.rateLimiter.Wait()

	fullURL := fmt.Sprintf("%s/api/v3/userDataStream?listenKey=%s", f.baseURL, listenKey)
	req, err := http.NewRequestWithContext(ctx, "PUT", fullURL, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("X-MBX-APIKEY", f.apiKey)

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("http status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}
