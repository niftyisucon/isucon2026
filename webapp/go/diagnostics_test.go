package isureal

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v5"
	"go.opentelemetry.io/otel"
)

func TestDiagnosticsOnlyBindsLoopback(t *testing.T) {
	for _, address := range []string{"", ":6060", "0.0.0.0:6060", "[::]:6060", "192.168.1.10:6060", "localhost:6060"} {
		if _, socket, err := NewDiagnosticsServer(address, nil); err == nil {
			socket.Close()
			t.Fatalf("accepted non-literal-loopback address %q", address)
		}
	}
	db, err := sql.Open("pgx", "postgres://private-user:private-password@127.0.0.1:1/private-database")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	server, socket, err := NewDiagnosticsServer("127.0.0.1:0", db)
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	go server.Serve(socket)
	response, err := http.Get("http://" + socket.Addr().String() + "/debug/stats")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil || response.StatusCode != 200 || !json.Valid(body) || strings.Contains(string(body), "private-") {
		t.Fatal("diagnostic stats failed or included DSN/credentials")
	}
	for _, path := range []string{"/debug/pprof/cmdline", "/debug/pprof/heap", "/debug/pprof/goroutine", "/debug/pprof/trace", "/debug/pprof/", "/api/me"} {
		recorder := httptest.NewRecorder()
		server.Handler.ServeHTTP(recorder, httptest.NewRequest("GET", path, nil))
		if recorder.Code != 404 {
			t.Fatalf("diagnostics exposed %s", path)
		}
	}
	for _, seconds := range []string{"0", "-1", "31", "bad"} {
		recorder := httptest.NewRecorder()
		server.Handler.ServeHTTP(recorder, httptest.NewRequest("GET", "/debug/pprof/profile?seconds="+seconds, nil))
		if recorder.Code != 400 {
			t.Fatal("unbounded CPU profile duration accepted")
		}
	}
}

func TestSessionTraceAndStatsExposeConnectionWait(t *testing.T) {
	fixtureDB, dsn := architectureDB(t)
	_, token := fixtureOwner(t, fixtureDB)
	exporter := testTraceExporter(t)
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Tracer = &queryTracer{tracer: otel.Tracer("test/postgresql")}
	db := sqlx.NewDb(stdlib.OpenDB(*cfg), "pgx")
	defer db.Close()
	db.SetMaxOpenConns(1)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	held, err := db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer held.Close()
	e := echo.New()
	e.Use(TracingMiddleware())
	e.GET("/probe", func(c *echo.Context) error { return c.NoContent(200) }, SessionMiddleware(&HandlerContext{DB: db}, true))
	req := httptest.NewRequest("GET", "/probe", nil).WithContext(ctx)
	req.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()
	done := make(chan struct{})
	go func() { defer close(done); e.ServeHTTP(recorder, req) }()
	for db.Stats().WaitCount == 0 {
		select {
		case <-ctx.Done():
			t.Fatal("request did not wait for the held pool connection")
		case <-time.After(time.Millisecond):
		}
	}
	statsRecorder := httptest.NewRecorder()
	diagnosticsHandler(db.DB).ServeHTTP(statsRecorder, httptest.NewRequest("GET", "/debug/stats", nil))
	var snapshot struct {
		DB sql.DBStats `json:"db"`
	}
	if err = json.Unmarshal(statsRecorder.Body.Bytes(), &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot.DB.WaitCount == 0 || snapshot.DB.InUse != 1 || snapshot.DB.Idle != 0 {
		t.Fatal("stats did not observe pool saturation")
	}
	held.Close()
	select {
	case <-done:
	case <-ctx.Done():
		t.Fatal("request did not finish after returning the connection")
	}
	if recorder.Code != 200 || db.Stats().WaitDuration <= 0 {
		t.Fatal("pool wait changed authentication or was not counted")
	}
	spans := exporter.GetSpans()
	for _, phase := range spans {
		if phase.Name != "session.lookup" {
			continue
		}
		for _, query := range spans {
			if strings.HasPrefix(query.Name, "postgresql SELECT ") && query.Parent.SpanID() == phase.SpanContext.SpanID() {
				if phase.StartTime.After(query.StartTime) || phase.EndTime.Before(query.EndTime) {
					t.Fatal("session phase failed to include connection acquisition and SQL")
				}
				return
			}
		}
	}
	t.Fatal("session lookup phase or its SQL child was missing")
}
