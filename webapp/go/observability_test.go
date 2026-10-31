package isureal

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/labstack/echo/v5"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func testTraceExporter(t *testing.T) *tracetest.InMemoryExporter {
	t.Helper()
	exporter := tracetest.NewInMemoryExporter()
	previous := otel.GetTracerProvider()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	otel.SetTracerProvider(provider)
	t.Cleanup(func() {
		_ = provider.Shutdown(context.Background())
		otel.SetTracerProvider(previous)
	})
	return exporter
}

func TestHTTPAndSQLTracesDoNotExportRequestOrQueryData(t *testing.T) {
	exporter := testTraceExporter(t)
	e := echo.New()
	e.Use(TracingMiddleware())
	e.GET("/api/users/:userId", func(c *echo.Context) error {
		tracer := &queryTracer{tracer: otel.Tracer("test/postgresql"), database: "isureal"}
		ctx := tracer.TraceQueryStart(c.Request().Context(), nil, pgx.TraceQueryStartData{
			SQL: "SELECT 'private-query-value' WHERE id=$1", Args: []any{"private-argument"},
		})
		tracer.TraceQueryEnd(ctx, nil, pgx.TraceQueryEndData{Err: &pgconn.PgError{
			Code: "23505", Message: "private-error-value", Detail: "private-detail-value",
		}})
		return c.NoContent(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodGet, "/api/users/private-user-id?token=private-query-token", nil)
	req.Header.Set("Authorization", "Bearer private-authorization-value")
	recorder := httptest.NewRecorder()
	e.ServeHTTP(recorder, req)
	spans := exporter.GetSpans()
	if recorder.Code != http.StatusOK || len(spans) != 2 {
		t.Fatalf("status=%d spans=%d", recorder.Code, len(spans))
	}
	sqlSpan, httpSpan := spans[0], spans[1]
	if httpSpan.Name != "GET /api/users/:userId" || sqlSpan.Parent.SpanID() != httpSpan.SpanContext.SpanID() {
		t.Fatal("SQL span must be a child of the templated HTTP route")
	}
	if sqlSpan.Status.Code != codes.Error {
		t.Fatal("database error must mark the SQL span as failed")
	}
	for _, span := range spans {
		if strings.Contains(fmt.Sprint(span.Name, span.Attributes, span.Events, span.Status), "private-") {
			t.Fatal("trace contains request data, SQL text, arguments, or database error details")
		}
	}
}

func TestHTTPTraceRecordsRenderedErrorStatus(t *testing.T) {
	exporter := testTraceExporter(t)
	e := echo.New()
	e.HTTPErrorHandler = APIErrorHandler
	e.Use(TracingMiddleware())
	e.Use(ActionLogMiddleware("error", io.Discard))
	e.GET("/fail", func(*echo.Context) error { return errors.New("private-error-message") })
	recorder := httptest.NewRecorder()
	e.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/fail", nil))
	spans := exporter.GetSpans()
	if recorder.Code != http.StatusInternalServerError || len(spans) != 1 || spans[0].Status.Code != codes.Error {
		t.Fatal("HTTP trace must reflect the rendered 500 response")
	}
}

func TestEdgeAndCoreShareTrace(t *testing.T) {
	exporter := testTraceExporter(t)
	core := echo.New()
	core.Use(TracingMiddleware())
	core.GET("/api/me", func(c *echo.Context) error { return c.NoContent(http.StatusOK) })
	upstream := httptest.NewServer(core)
	defer upstream.Close()
	edge := echo.New()
	edge.Use(TracingMiddleware())
	edge.Any("/api/*", newEdgeProxy(&Config{EdgeUpstream: upstream.URL, EdgeKey: architectureTestKey}, NewImageCache(imageCacheMaxBytes)))
	recorder := httptest.NewRecorder()
	edge.ServeHTTP(recorder, httptest.NewRequest("GET", "/api/me", nil))
	if recorder.Code != http.StatusOK {
		t.Fatal("trace propagation changed proxy response")
	}
	spans := exporter.GetSpans()
	if len(spans) != 2 || spans[0].Name != "GET /api/me" || spans[1].Name != "GET /api/*" ||
		spans[0].Parent.SpanID() != spans[1].SpanContext.SpanID() ||
		spans[0].SpanContext.TraceID() != spans[1].SpanContext.TraceID() {
		t.Fatal("edge/core were exported as disconnected traces")
	}
}

func TestInvalidTracingRatioIsRejected(t *testing.T) {
	for _, ratio := range []string{"NaN", "-1", "1.1", "invalid"} {
		t.Run(ratio, func(t *testing.T) {
			t.Setenv("ISUREAL_TRACE_SAMPLE_RATIO", ratio)
			if _, err := StartTracing(context.Background(), "asia"); err == nil {
				t.Fatal("invalid sampling ratio accepted")
			}
		})
	}
}

func TestPostgreSQLTracingIntegration(t *testing.T) {
	if os.Getenv("ISUREAL_TEST_POSTGRES") != "1" {
		t.Skip("set ISUREAL_TEST_POSTGRES=1 with a local PostgreSQL 16 instance")
	}
	exporter := testTraceExporter(t)
	config := ConfigFromEnv()
	config.TracingEnabled = true
	db, err := openDatabase(config)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx, parent := otel.Tracer("test/http").Start(context.Background(), "test request")
	var value int
	if err := db.GetContext(ctx, &value, "SELECT $1::integer", 42); err != nil {
		t.Fatal(err)
	}
	parent.End()
	if value != 42 {
		t.Fatal("instrumented database changed the query result")
	}
	for _, span := range exporter.GetSpans() {
		if strings.HasPrefix(span.Name, "postgresql SELECT ") && span.Parent.SpanID() == parent.SpanContext().SpanID() {
			return
		}
	}
	t.Fatal("pgx/database/sql query was not exported as a child of the request")
}
