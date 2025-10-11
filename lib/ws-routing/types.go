package wsrouting

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/coachpo/meltica/errs"
	"github.com/coachpo/meltica/internal/observability"
)

// State represents the lifecycle phase of a framework session.
type State string

const (
	// StateUnknown indicates the lifecycle could not be determined.
	StateUnknown State = "unknown"
	// StateInitialized indicates the session has been constructed but not started.
	StateInitialized State = "initialized"
	// StateStarting indicates the session is dialing the underlying connection.
	StateStarting State = "starting"
	// StateStreaming indicates the session is actively streaming data.
	StateStreaming State = "streaming"
	// StateStopping indicates the session is shutting down.
	StateStopping State = "stopping"
	// StateStopped indicates the session has fully terminated.
	StateStopped State = "stopped"
)

// BackoffConfig defines retry behaviour for reconnect attempts.
type BackoffConfig struct {
	Initial    time.Duration
	Max        time.Duration
	Multiplier *big.Rat
	Jitter     int
}

func (cfg BackoffConfig) validate() (BackoffConfig, error) {
	if cfg.Initial <= 0 {
		return BackoffConfig{}, errs.New("", errs.CodeInvalid, errs.WithMessage("backoff initial delay must be positive"))
	}
	if cfg.Max > 0 && cfg.Max < cfg.Initial {
		return BackoffConfig{}, errs.New("", errs.CodeInvalid, errs.WithMessage("backoff max delay must be >= initial"))
	}
	if cfg.Jitter < 0 || cfg.Jitter > 100 {
		return BackoffConfig{}, errs.New("", errs.CodeInvalid, errs.WithMessage("backoff jitter must be between 0 and 100"))
	}
	clone := BackoffConfig{
		Initial: cfg.Initial,
		Max:     cfg.Max,
		Jitter:  cfg.Jitter,
	}
	if cfg.Multiplier == nil {
		clone.Multiplier = big.NewRat(1, 1)
	} else if cfg.Multiplier.Sign() <= 0 {
		return BackoffConfig{}, errs.New("", errs.CodeInvalid, errs.WithMessage("backoff multiplier must be positive"))
	} else {
		clone.Multiplier = new(big.Rat).Set(cfg.Multiplier)
	}
	return clone, nil
}

func (cfg BackoffConfig) multiplier() rational {
	if cfg.Multiplier == nil {
		return rational{num: 1, den: 1}
	}
	return newRational(cfg.Multiplier)
}

type rational struct {
	num int64
	den int64
}

func newRational(r *big.Rat) rational {
	if r == nil {
		return rational{num: 1, den: 1}
	}
	n := r.Num()
	d := r.Denom()
	if d.Sign() == 0 {
		return rational{num: 1, den: 1}
	}
	return rational{num: n.Int64(), den: d.Int64()}
}

// Options configures a framework session.
type Options struct {
	SessionID string
	Dialer    Dialer
	Parser    Parser
	Publish   PublishFunc
	Backoff   BackoffConfig
	Logger    observability.Logger
}

func (opts Options) validate() (Options, error) {
	id := strings.TrimSpace(opts.SessionID)
	if id == "" {
		return Options{}, errs.New("", errs.CodeInvalid, errs.WithMessage("session id required"))
	}
	if opts.Dialer == nil {
		return Options{}, errs.New("", errs.CodeInvalid, errs.WithMessage("dialer required"))
	}
	if opts.Parser == nil {
		return Options{}, errs.New("", errs.CodeInvalid, errs.WithMessage("parser required"))
	}
	if opts.Publish == nil {
		return Options{}, errs.New("", errs.CodeInvalid, errs.WithMessage("publish function required"))
	}
	backoff, err := opts.Backoff.validate()
	if err != nil {
		return Options{}, err
	}
	logger := opts.Logger
	if logger == nil {
		logger = observability.Log()
	}
	return Options{
		SessionID: id,
		Dialer:    opts.Dialer,
		Parser:    opts.Parser,
		Publish:   opts.Publish,
		Backoff:   backoff,
		Logger:    logger,
	}, nil
}

// DialOptions provides connection context to dialers.
type DialOptions struct {
	SessionID string
	Backoff   BackoffConfig
	Logger    observability.Logger
}

// Dialer establishes transport connections for sessions.
type Dialer interface {
	Dial(context.Context, DialOptions) (Connection, error)
}

// Parser converts raw exchange payloads into messages.
type Parser interface {
	Parse(context.Context, []byte) (*Message, error)
}

// PublishFunc delivers processed messages to domain adapters.
type PublishFunc func(context.Context, *Message) error

// Connection represents a streaming transport connection.
type Connection interface {
	Subscribe(context.Context, SubscriptionSpec) error
	Close(context.Context) error
}

// Message represents a normalized routing payload.
type Message struct {
	Type       string
	Payload    map[string]any
	Metadata   map[string]string
	ReceivedAt time.Time
}

// Middleware intercepts routed messages before publication.
type Middleware func(context.Context, *Message) (*Message, error)

// SubscriptionSpec describes a routed subscription.
type SubscriptionSpec struct {
	Exchange string
	Channel  string
	Symbols  []string
	QoS      QoSLevel
}

// QoSLevel enumerates quality of service options.
type QoSLevel string

const (
	// QoSRealtime delivers live streaming data.
	QoSRealtime QoSLevel = "realtime"
	// QoSSnapshot delivers a single snapshot.
	QoSSnapshot QoSLevel = "snapshot"
)

// Validate ensures the subscription describes a valid stream.
func (s SubscriptionSpec) Validate() error {
	if strings.TrimSpace(s.Exchange) == "" {
		return errs.New("", errs.CodeInvalid, errs.WithMessage("exchange required"))
	}
	if strings.TrimSpace(s.Channel) == "" {
		return errs.New("", errs.CodeInvalid, errs.WithMessage("channel required"))
	}
	if len(s.Symbols) == 0 {
		return errs.New("", errs.CodeInvalid, errs.WithMessage("symbols required"))
	}
	for _, sym := range s.Symbols {
		sanitized := strings.TrimSpace(sym)
		if sanitized == "" {
			return errs.New("", errs.CodeInvalid, errs.WithMessage("symbols cannot contain empty entries"))
		}
		if strings.Count(sanitized, "-") != 1 {
			return errs.New("", errs.CodeInvalid, errs.WithMessage(fmt.Sprintf("symbol %s must follow BASE-QUOTE format", sanitized)))
		}
	}
	if s.QoS != "" && s.QoS != QoSRealtime && s.QoS != QoSSnapshot {
		return errs.New("", errs.CodeInvalid, errs.WithMessage("unsupported qos level"))
	}
	return nil
}

// Key returns a stable identifier for deduplicating subscriptions.
func (s SubscriptionSpec) Key() string {
	symbols := make([]string, 0, len(s.Symbols))
	for _, sym := range s.Symbols {
		symbols = append(symbols, strings.TrimSpace(sym))
	}
	return fmt.Sprintf("%s|%s|%s|%s", strings.TrimSpace(s.Exchange), strings.TrimSpace(s.Channel), strings.Join(symbols, ","), string(s.QoS))
}
