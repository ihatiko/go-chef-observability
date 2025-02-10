package tracer

import (
	"context"
	"log"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	exporter "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
)

const (
	commandKey = "command"
)

type Options func(*Tracer)

type Tracer struct {
	ServiceName string `toml:"service_name"`
	Command     string `toml:"command"`
	Deployment  string `toml:"deployment"`
}

func WithDeployment(name string) Options {
	return func(tracer *Tracer) {
		tracer.Deployment = name
	}
}

func WithCommand(name string) Options {
	return func(tracer *Tracer) {
		tracer.Command = name
	}
}

func WithServiceName(name string) Options {
	return func(tracer *Tracer) {
		tracer.ServiceName = name
	}
}

const deploymentKey = "deployments"

func (cfg *Config) New(opts ...Options) {
	tracer := new(Tracer)
	if cfg.Ratio == 0 {
		cfg.Ratio = 0.01
	}

	for _, opt := range opts {
		opt(tracer)
	}
	if tracer.ServiceName == "" {
		tracer.ServiceName = os.Getenv("TECH.SERVICE.NAME")
	}
	if tracer.Deployment == "" {
		tracer.Deployment = os.Getenv("TECH.SERVICE.DEPLOYMENT")
	}

	exp, err := exporter.New(context.Background(),
		exporter.WithEndpoint(cfg.Host),
	)
	if err != nil {
		log.Fatal(err)
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(
			resource.NewWithAttributes(
				semconv.SchemaURL,
				semconv.ServiceNameKey.String(tracer.ServiceName),
				attribute.String(commandKey, tracer.Command),
				attribute.String(deploymentKey, tracer.Deployment),
			),
		),
		sdktrace.WithSampler(
			sdktrace.TraceIDRatioBased(cfg.Ratio),
		),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{}, propagation.Baggage{}),
	)
}
