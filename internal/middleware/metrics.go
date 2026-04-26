package middleware

import (
	"log"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var (
	requestCounter  metric.Int64Counter
	requestDuration metric.Float64Histogram
)

// InitMetrics создаёт счётчики из глобального OTel MeterProvider. Вызывается
// в main ПОСЛЕ observability.Setup — иначе провайдер ещё не выставлен и
// инструменты получатся no-op.
func InitMetrics() {
	meter := otel.Meter("diploma-api")
	var err error
	requestCounter, err = meter.Int64Counter(
		"documents_requests_total",
		metric.WithDescription("Общее число HTTP-запросов, обработанных API"),
	)
	if err != nil {
		log.Fatalf("init metrics counter: %v", err)
	}

	requestDuration, err = meter.Float64Histogram(
		"documents_request_duration_seconds",
		metric.WithDescription("Длительность HTTP-запросов в секундах"),
		metric.WithUnit("s"),
	)
	if err != nil {
		log.Fatalf("init metrics histogram: %v", err)
	}
}

// Metrics — gin-middleware, считает каждый запрос. Если InitMetrics не была
// вызвана (счётчики nil), middleware просто пропускает запрос дальше — на
// случай тестов или нестандартного запуска.
func Metrics() gin.HandlerFunc {
	return func(c *gin.Context) {
		if requestCounter == nil {
			c.Next()
			return
		}

		start := time.Now()
		c.Next()

		// FullPath даёт зарегистрированный шаблон (/documents/:id), а не сырой
		// URL — иначе кардинальность лейбла взорвётся.
		path := c.FullPath()
		if path == "" {
			path = "unknown"
		}
		status := strconv.Itoa(c.Writer.Status())

		requestCounter.Add(c.Request.Context(), 1,
			metric.WithAttributes(
				attribute.String("method", c.Request.Method),
				attribute.String("path", path),
				attribute.String("status", status),
			))
		requestDuration.Record(c.Request.Context(), time.Since(start).Seconds(),
			metric.WithAttributes(
				attribute.String("method", c.Request.Method),
				attribute.String("path", path),
			))
	}
}
