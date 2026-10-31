package isureal

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v5"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

func StartTracing(ctx context.Context, region string) (func(context.Context) error, error) {
	ratio, err := strconv.ParseFloat(envOrDefault("ISUREAL_TRACE_SAMPLE_RATIO", "0.01"), 64)
	if err != nil || !(ratio >= 0 && ratio <= 1) {
		return nil, errors.New("ISUREAL_TRACE_SAMPLE_RATIO must be between 0 and 1")
	}
	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}
	res, err := resource.New(ctx,
		resource.WithAttributes(
			attribute.String("service.name", "isureal"),
			attribute.String("service.instance.id", hostname),
			attribute.String("deployment.environment.name", "isucon2026"),
			attribute.String("isureal.region", region),
		),
		resource.WithFromEnv(), resource.WithHost(),
	)
	if err != nil {
		return nil, err
	}
	exporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpointURL(envOrDefault("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", "http://127.0.0.1:4318/v1/traces")),
		otlptracehttp.WithTimeout(2*time.Second),
	)
	if err != nil {
		return nil, err
	}
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(ratio))),
		sdktrace.WithBatcher(exporter,
			sdktrace.WithMaxQueueSize(2048),
			sdktrace.WithMaxExportBatchSize(256),
			sdktrace.WithBatchTimeout(time.Second),
			sdktrace.WithExportTimeout(2*time.Second),
		),
	)
	otel.SetTracerProvider(provider)
	return provider.Shutdown, nil
}

func TracingMiddleware() echo.MiddlewareFunc {
	tracer := otel.Tracer("isureal/http")
	propagator := propagation.TraceContext{}
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			req := c.Request()
			route := c.RouteInfo().Path
			if route == "" {
				route = "unmatched"
			}
			ctx := propagator.Extract(req.Context(), propagation.HeaderCarrier(req.Header))
			ctx, span := tracer.Start(ctx, req.Method+" "+route,
				trace.WithSpanKind(trace.SpanKindServer),
				trace.WithAttributes(attribute.String("http.request.method", req.Method), attribute.String("http.route", route)),
			)
			defer span.End()
			c.SetRequest(req.WithContext(ctx))
			err := next(c)
			_, status := echo.ResolveResponseStatus(c.Response(), err)
			span.SetAttributes(attribute.Int("http.response.status_code", status))
			if status >= http.StatusInternalServerError {
				span.SetStatus(codes.Error, "HTTP server error")
			}
			return err
		}
	}
}

func openDatabase(config *Config) (*sqlx.DB, error) {
	if !config.TracingEnabled {
		return sqlx.Open("pgx", config.DatabaseDSN())
	}
	dbConfig, err := pgx.ParseConfig(config.DatabaseDSN())
	if err != nil {
		return nil, err
	}
	dbConfig.Tracer = &queryTracer{
		tracer: otel.Tracer("isureal/postgresql"), database: config.DBName,
		address: dbConfig.Host, port: int(dbConfig.Port),
	}
	return sqlx.NewDb(stdlib.OpenDB(*dbConfig), "pgx"), nil
}

type queryTracer struct {
	tracer   trace.Tracer
	database string
	address  string
	port     int
}

func (t *queryTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	ctx, span := t.tracer.Start(ctx, "postgresql", trace.WithSpanKind(trace.SpanKindClient))
	if span.IsRecording() {
		operation := "QUERY"
		if fields := strings.Fields(data.SQL); len(fields) > 0 {
			switch verb := strings.ToUpper(fields[0]); verb {
			case "SELECT", "INSERT", "UPDATE", "DELETE", "WITH", "BEGIN", "COMMIT", "ROLLBACK":
				operation = verb
			}
		}
		digest := sha256.Sum256([]byte(data.SQL))
		fingerprint := hex.EncodeToString(digest[:8])
		span.SetName("postgresql " + operation + " " + fingerprint)
		span.SetAttributes(
			attribute.String("db.system", "postgresql"),
			attribute.String("db.system.name", "postgresql"),
			attribute.String("db.name", t.database),
			attribute.String("db.namespace", t.database),
			attribute.String("server.address", t.address),
			attribute.Int("server.port", t.port),
			attribute.String("db.operation.name", operation),
			attribute.String("db.query.fingerprint", fingerprint),
		)
	}
	return ctx
}

func (t *queryTracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	span := trace.SpanFromContext(ctx)
	if data.Err != nil && span.IsRecording() {
		span.SetStatus(codes.Error, "database operation failed")
		var pgErr *pgconn.PgError
		if errors.As(data.Err, &pgErr) {
			span.SetAttributes(attribute.String("db.response.status_code", pgErr.Code))
		} else {
			span.SetAttributes(attribute.String("error.type", fmt.Sprintf("%T", data.Err)))
		}
	}
	span.End()
}
