// Package telemetry provides OpenTelemetry initialization and instrumentation.
package telemetry

import (
	"context"
	"fmt"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.32.0"
	"go.opentelemetry.io/otel/trace"
)

const (
	serviceName    = "meltica"
	serviceVersion = "1.0.0"
)

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
}

func DefaultConfig() Config {
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		endpoint = "localhost:4318"
	}
	svcName := os.Getenv("OTEL_SERVICE_NAME")
	if svcName == "" {
		svcName = serviceName
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
	}
}

type Provider struct {
	tracerProvider *sdktrace.TracerProvider
	meterProvider  *sdkmetric.MeterProvider
	config         Config
}

func NewProvider(ctx context.Context, cfg Config) (*Provider, error) {
	if !cfg.Enabled {
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
		tp.Shutdown(ctx)
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

func (p *Provider) Tracer(name string, opts ...trace.TracerOption) trace.Tracer {
	if p.tracerProvider == nil {
		return otel.Tracer(name, opts...)
	}
	return p.tracerProvider.Tracer(name, opts...)
}

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
	attrs = append(attrs, resource.WithProcessRuntimeName())
	attrs = append(attrs, resource.WithProcessRuntimeVersion())
	attrs = append(attrs, resource.WithHost())
	return resource.New(ctx, attrs...)
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

	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter,
			sdkmetric.WithInterval(cfg.MetricInterval),
		)),
	)
	return mp, nil
}
