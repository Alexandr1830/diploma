// Package observability настраивает экспорт трейсов, метрик и логов через
// OTLP HTTP в коллектор. По умолчанию ожидает grafana/otel-lgtm на :4318
// (через переменную OTEL_EXPORTER_OTLP_ENDPOINT).
package observability

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.34.0"
)

// Shutdown — функция graceful shutdown всех экспортёров. Вызывать через defer.
type Shutdown func(context.Context) error

// teeHandler дублирует записи slog в два handler-а: один пишет JSON в stdout
// (видно в docker logs), второй отправляет в OTLP коллектор (попадает в Loki).
type teeHandler struct {
	a, b slog.Handler
}

func (t teeHandler) Enabled(ctx context.Context, l slog.Level) bool {
	return t.a.Enabled(ctx, l) || t.b.Enabled(ctx, l)
}

func (t teeHandler) Handle(ctx context.Context, r slog.Record) error {
	_ = t.a.Handle(ctx, r)
	return t.b.Handle(ctx, r)
}

func (t teeHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return teeHandler{t.a.WithAttrs(attrs), t.b.WithAttrs(attrs)}
}

func (t teeHandler) WithGroup(name string) slog.Handler {
	return teeHandler{t.a.WithGroup(name), t.b.WithGroup(name)}
}

// Setup инициализирует tracer, meter и slog-bridge с OTLP-экспортом.
// Если коллектор недоступен, экспорт фейлится в фоне, приложение не падает.
func Setup(ctx context.Context, serviceName string) (Shutdown, error) {
	// NewSchemaless избегает конфликта Schema URL с resource.Default(),
	// который SDK периодически бампит. Сервисное имя — единственное, что
	// нам реально нужно: по нему datasources в Grafana группируют данные.
	res := resource.NewSchemaless(semconv.ServiceName(serviceName))

	traceExp, err := otlptracehttp.New(ctx, otlptracehttp.WithInsecure())
	if err != nil {
		return nil, fmt.Errorf("observability: trace exporter: %w", err)
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExp),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	metricExp, err := otlpmetrichttp.New(ctx, otlpmetrichttp.WithInsecure())
	if err != nil {
		return nil, fmt.Errorf("observability: metric exporter: %w", err)
	}
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExp,
			sdkmetric.WithInterval(5*time.Second))),
		sdkmetric.WithResource(res),
	)
	otel.SetMeterProvider(mp)

	logExp, err := otlploghttp.New(ctx, otlploghttp.WithInsecure())
	if err != nil {
		return nil, fmt.Errorf("observability: log exporter: %w", err)
	}
	lp := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(logExp)),
		sdklog.WithResource(res),
	)

	stdoutHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	otelHandler := otelslog.NewHandler(serviceName, otelslog.WithLoggerProvider(lp))
	slog.SetDefault(slog.New(teeHandler{a: stdoutHandler, b: otelHandler}))

	return func(c context.Context) error {
		_ = tp.Shutdown(c)
		_ = mp.Shutdown(c)
		_ = lp.Shutdown(c)
		return nil
	}, nil
}

// MustSetup — обёртка для main: при ошибке инициализации не валит процесс,
// просто пишет предупреждение в stderr. Возвращает no-op shutdown.
func MustSetup(serviceName string) Shutdown {
	sd, err := Setup(context.Background(), serviceName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "observability disabled: %v\n", err)
		return func(context.Context) error { return nil }
	}
	return sd
}
