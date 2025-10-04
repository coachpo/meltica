package config

import (
	"os"
	"strings"
	"time"
)

type Environment string

const (
	EnvDev     Environment = "dev"
	EnvStaging Environment = "staging"
	EnvProd    Environment = "prod"
)

type BinanceSettings struct {
	SpotBaseURL      string
	LinearBaseURL    string
	InverseBaseURL   string
	PublicWSURL      string
	PrivateWSURL     string
	HTTPTimeout      time.Duration
	HandshakeTimeout time.Duration
	APIKey           string
	APISecret        string
}

type Settings struct {
	Environment Environment
	Binance     BinanceSettings
}

func Default() Settings {
	return Settings{
		Environment: EnvProd,
		Binance: BinanceSettings{
			SpotBaseURL:      "https://api.binance.com",
			LinearBaseURL:    "https://fapi.binance.com",
			InverseBaseURL:   "https://dapi.binance.com",
			PublicWSURL:      "wss://stream.binance.com:9443/stream",
			PrivateWSURL:     "wss://stream.binance.com:9443/ws",
			HTTPTimeout:      10 * time.Second,
			HandshakeTimeout: 10 * time.Second,
		},
	}
}

func FromEnv() Settings {
	cfg := Default()
	if env := strings.TrimSpace(os.Getenv("MELTICA_ENV")); env != "" {
		cfg.Environment = Environment(strings.ToLower(env))
	}
	if v := strings.TrimSpace(os.Getenv("BINANCE_SPOT_BASE_URL")); v != "" {
		cfg.Binance.SpotBaseURL = v
	}
	if v := strings.TrimSpace(os.Getenv("BINANCE_LINEAR_BASE_URL")); v != "" {
		cfg.Binance.LinearBaseURL = v
	}
	if v := strings.TrimSpace(os.Getenv("BINANCE_INVERSE_BASE_URL")); v != "" {
		cfg.Binance.InverseBaseURL = v
	}
	if v := strings.TrimSpace(os.Getenv("BINANCE_WS_PUBLIC_URL")); v != "" {
		cfg.Binance.PublicWSURL = v
	}
	if v := strings.TrimSpace(os.Getenv("BINANCE_WS_PRIVATE_URL")); v != "" {
		cfg.Binance.PrivateWSURL = v
	}
	if v := strings.TrimSpace(os.Getenv("BINANCE_HTTP_TIMEOUT")); v != "" {
		if dur, err := time.ParseDuration(v); err == nil {
			cfg.Binance.HTTPTimeout = dur
		}
	}
	if v := strings.TrimSpace(os.Getenv("BINANCE_WS_HANDSHAKE_TIMEOUT")); v != "" {
		if dur, err := time.ParseDuration(v); err == nil {
			cfg.Binance.HandshakeTimeout = dur
		}
	}
	if v := strings.TrimSpace(os.Getenv("BINANCE_API_KEY")); v != "" {
		cfg.Binance.APIKey = v
	}
	if v := strings.TrimSpace(os.Getenv("BINANCE_API_SECRET")); v != "" {
		cfg.Binance.APISecret = v
	}
	return cfg
}

type Option func(*Settings)

func Apply(base Settings, opts ...Option) Settings {
	cfg := base
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	return cfg
}

func WithEnvironment(env Environment) Option {
	return func(s *Settings) {
		if env != "" {
			s.Environment = env
		}
	}
}

func WithBinanceRESTEndpoints(spot, linear, inverse string) Option {
	return func(s *Settings) {
		if strings.TrimSpace(spot) != "" {
			s.Binance.SpotBaseURL = spot
		}
		if strings.TrimSpace(linear) != "" {
			s.Binance.LinearBaseURL = linear
		}
		if strings.TrimSpace(inverse) != "" {
			s.Binance.InverseBaseURL = inverse
		}
	}
}

func WithBinanceWebsocketEndpoints(public, private string, handshake time.Duration) Option {
	return func(s *Settings) {
		if strings.TrimSpace(public) != "" {
			s.Binance.PublicWSURL = public
		}
		if strings.TrimSpace(private) != "" {
			s.Binance.PrivateWSURL = private
		}
		if handshake > 0 {
			s.Binance.HandshakeTimeout = handshake
		}
	}
}

func WithBinanceHTTPTimeout(timeout time.Duration) Option {
	return func(s *Settings) {
		if timeout > 0 {
			s.Binance.HTTPTimeout = timeout
		}
	}
}

func WithBinanceAPI(key, secret string) Option {
	return func(s *Settings) {
		s.Binance.APIKey = strings.TrimSpace(key)
		s.Binance.APISecret = strings.TrimSpace(secret)
	}
}
