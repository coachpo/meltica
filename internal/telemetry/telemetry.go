// Package telemetry provides OpenTelemetry initialization and instrumentation.
package telemetry

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	instrumentationsdk "go.opentelemetry.io/otel/sdk/instrumentation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.32.0"
	"go.opentelemetry.io/otel/trace"
)

const (
	serviceName    = "meltica"
	serviceVersion = "1.0.0"
)

var (
	// globalEnvironment stores the environment name for use in metric labels
	globalEnvironment string
)

// Config defines OpenTelemetry configuration parameters.
type Config struct {
	Enabled          bool
	OTLPEndpoint     string
	OTLPInsecure     bool
	SampleRate       float64
	MetricInterval   time.Duration
	ShutdownTimeout  time.Duration
	ConsoleExporter  bool
	ServiceName      string
	ServiceVersion   string
	ServiceNamespace string
	Environment      string
}

// DefaultConfig returns the default telemetry configuration based on environment variables.
func DefaultConfig() Config {
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		endpoint = "localhost:4318"
	}
	svcName := os.Getenv("OTEL_SERVICE_NAME")
	if svcName == "" {
		svcName = serviceName
	}
	env := strings.TrimSpace(os.Getenv("OTEL_RESOURCE_ENVIRONMENT"))
	if env == "" {
		env = strings.TrimSpace(os.Getenv("MELTICA_ENV"))
	}
	if env == "" {
		env = "development"
	}
	return Config{
		Enabled:          os.Getenv("OTEL_ENABLED") != "false",
		OTLPEndpoint:     endpoint,
		OTLPInsecure:     os.Getenv("OTEL_EXPORTER_OTLP_INSECURE") == "true",
		SampleRate:       1.0,
		MetricInterval:   30 * time.Second,
		ShutdownTimeout:  5 * time.Second,
		ConsoleExporter:  os.Getenv("OTEL_CONSOLE_EXPORTER") == "true",
		ServiceName:      svcName,
		ServiceVersion:   serviceVersion,
		ServiceNamespace: os.Getenv("OTEL_SERVICE_NAMESPACE"),
		Environment:      env,
	}
}

// Provider manages OpenTelemetry tracer and meter providers.
type Provider struct {
	tracerProvider *sdktrace.TracerProvider
	meterProvider  *sdkmetric.MeterProvider
	config         Config
}

// NewProvider initializes a new telemetry provider with the given configuration.
func NewProvider(ctx context.Context, cfg Config) (*Provider, error) {
	// Set global environment for metric labels
	globalEnvironment = strings.ToLower(cfg.Environment)
	
	if !cfg.Enabled {
		//nolint:exhaustruct // zero values for tracerProvider and meterProvider when disabled
		return &Provider{config: cfg}, nil
	}

	res, err := newResource(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create resource: %w", err)
	}

	tp, err := newTracerProvider(ctx, res, cfg)
	if err != nil {
		return nil, fmt.Errorf("create tracer provider: %w", err)
	}

	mp, err := newMeterProvider(ctx, res, cfg)
	if err != nil {
		if shutdownErr := tp.Shutdown(ctx); shutdownErr != nil {
			return nil, fmt.Errorf("create meter provider: %w (shutdown error: %v)", err, shutdownErr)
		}
		return nil, fmt.Errorf("create meter provider: %w", err)
	}

	otel.SetTracerProvider(tp)
	otel.SetMeterProvider(mp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return &Provider{
		tracerProvider: tp,
		meterProvider:  mp,
		config:         cfg,
	}, nil
}

// Shutdown gracefully shuts down the telemetry provider.
func (p *Provider) Shutdown(ctx context.Context) error {
	if p.tracerProvider == nil && p.meterProvider == nil {
		return nil
	}
	var err error
	if p.tracerProvider != nil {
		if tErr := p.tracerProvider.Shutdown(ctx); tErr != nil {
			err = fmt.Errorf("shutdown tracer: %w", tErr)
		}
	}
	if p.meterProvider != nil {
		if mErr := p.meterProvider.Shutdown(ctx); mErr != nil {
			if err != nil {
				err = fmt.Errorf("%w; shutdown meter: %w", err, mErr)
			} else {
				err = fmt.Errorf("shutdown meter: %w", mErr)
			}
		}
	}
	return err
}

// Tracer returns a tracer with the given name.
func (p *Provider) Tracer(name string, opts ...trace.TracerOption) trace.Tracer {
	if p.tracerProvider == nil {
		return otel.Tracer(name, opts...)
	}
	return p.tracerProvider.Tracer(name, opts...)
}

// Meter returns a meter with the given name.
func (p *Provider) Meter(name string, opts ...metric.MeterOption) metric.Meter {
	if p.meterProvider == nil {
		return otel.Meter(name, opts...)
	}
	return p.meterProvider.Meter(name, opts...)
}

func newResource(ctx context.Context, cfg Config) (*resource.Resource, error) {
	attrs := []resource.Option{
		resource.WithAttributes(
			semconv.ServiceNameKey.String(cfg.ServiceName),
			semconv.ServiceVersionKey.String(cfg.ServiceVersion),
		),
	}
	if cfg.ServiceNamespace != "" {
		attrs = append(attrs, resource.WithAttributes(
			semconv.ServiceNamespaceKey.String(cfg.ServiceNamespace),
		))
	}
	if cfg.Environment != "" {
		attrs = append(attrs, resource.WithAttributes(
			attribute.String("environment", strings.ToLower(cfg.Environment)),
		))
	}
	attrs = append(attrs, resource.WithProcessRuntimeName())
	attrs = append(attrs, resource.WithProcessRuntimeVersion())
	attrs = append(attrs, resource.WithHost())
	res, err := resource.New(ctx, attrs...)
	if err != nil {
		return nil, fmt.Errorf("create telemetry resource: %w", err)
	}
	return res, nil
}

func newTracerProvider(ctx context.Context, res *resource.Resource, cfg Config) (*sdktrace.TracerProvider, error) {
	exporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(cfg.OTLPEndpoint),
		otlptracehttp.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("create trace exporter: %w", err)
	}

	sampler := sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.SampleRate))
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithBatcher(exporter),
		sdktrace.WithSampler(sampler),
	)
	return tp, nil
}

func newMeterProvider(ctx context.Context, res *resource.Resource, cfg Config) (*sdkmetric.MeterProvider, error) {
	exporter, err := otlpmetrichttp.New(ctx,
		otlpmetrichttp.WithEndpoint(cfg.OTLPEndpoint),
		otlpmetrichttp.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("create metric exporter: %w", err)
	}

	// Configure Views for histogram bucket customization
	views := createHistogramViews()

	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter,
			sdkmetric.WithInterval(cfg.MetricInterval),
		)),
		sdkmetric.WithView(views...),
	)
	return mp, nil
}

// createHistogramViews configures explicit histogram buckets optimized for observed latency patterns.
func createHistogramViews() []sdkmetric.View {
	return []sdkmetric.View{
		// Dispatcher processing duration: 0.1ms - 500ms (event processing latency)
		sdkmetric.NewView(
			sdkmetric.Instrument{
				Name:        "dispatcher.processing.duration",
				Description: "Dispatcher processing duration",
				Kind:        sdkmetric.InstrumentKindHistogram,
				Unit:        "ms",
				Scope: instrumentationsdk.Scope{
					Name:       "",
					Version:    "",
					SchemaURL:  "",
					Attributes: attribute.Set{},
				},
			},
			sdkmetric.Stream{
				Name:        "",
				Description: "",
				Unit:        "",
				Aggregation: sdkmetric.AggregationExplicitBucketHistogram{
					Boundaries: []float64{0.1, 0.5, 1, 2, 5, 10, 25, 50, 100, 250, 500},
					NoMinMax:   false,
				},
				AttributeFilter:                   nil,
				ExemplarReservoirProviderSelector: nil,
			},
		),
		// Pool borrow duration: 0.01ms - 50ms (memory pool operations)
		sdkmetric.NewView(
			sdkmetric.Instrument{
				Name:        "pool.borrow.duration",
				Description: "Pool borrow operation duration",
				Kind:        sdkmetric.InstrumentKindHistogram,
				Unit:        "ms",
				Scope: instrumentationsdk.Scope{
					Name:       "",
					Version:    "",
					SchemaURL:  "",
					Attributes: attribute.Set{},
				},
			},
			sdkmetric.Stream{
				Name:        "",
				Description: "",
				Unit:        "",
				Aggregation: sdkmetric.AggregationExplicitBucketHistogram{
					Boundaries: []float64{0.01, 0.05, 0.1, 0.5, 1, 2, 5, 10, 25, 50},
					NoMinMax:   false,
				},
				AttributeFilter:                   nil,
				ExemplarReservoirProviderSelector: nil,
			},
		),
		// Orderbook cold start duration: 100ms - 30s (initial snapshot loading)
		sdkmetric.NewView(
			sdkmetric.Instrument{
				Name:        "orderbook.coldstart.duration",
				Description: "Orderbook cold start duration",
				Kind:        sdkmetric.InstrumentKindHistogram,
				Unit:        "ms",
				Scope: instrumentationsdk.Scope{
					Name:       "",
					Version:    "",
					SchemaURL:  "",
					Attributes: attribute.Set{},
				},
			},
			sdkmetric.Stream{
				Name:        "",
				Description: "",
				Unit:        "",
				Aggregation: sdkmetric.AggregationExplicitBucketHistogram{
					Boundaries: []float64{100, 250, 500, 1000, 2000, 5000, 10000, 30000},
					NoMinMax:   false,
				},
				AttributeFilter:                   nil,
				ExemplarReservoirProviderSelector: nil,
			},
		),
		// Databus fanout size: 1 - 100 subscribers
		sdkmetric.NewView(
			sdkmetric.Instrument{
				Name:        "databus.fanout.size",
				Description: "Databus fanout subscriber count",
				Kind:        sdkmetric.InstrumentKindHistogram,
				Unit:        "1",
				Scope: instrumentationsdk.Scope{
					Name:       "",
					Version:    "",
					SchemaURL:  "",
					Attributes: attribute.Set{},
				},
			},
			sdkmetric.Stream{
				Name:        "",
				Description: "",
				Unit:        "",
				Aggregation: sdkmetric.AggregationExplicitBucketHistogram{
					Boundaries: []float64{1, 2, 5, 10, 20, 50, 100},
					NoMinMax:   false,
				},
				AttributeFilter:                   nil,
				ExemplarReservoirProviderSelector: nil,
			},
		),
	}
}

// Environment returns the configured environment name for use in metric labels.
func Environment() string {
	if globalEnvironment == "" {
		return "development"
	}
	return globalEnvironment
}
