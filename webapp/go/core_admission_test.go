package isureal

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/labstack/echo/v5"
)

func admissionTestConfig() *Config {
	return &Config{Region: "asia", ListenRegions: "asia:8080,europe:8081,america:8082", EdgeKey: architectureTestKey, LogLevel: "error"}
}

func waitAdmission(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.NewTimer(3 * time.Second)
	defer deadline.Stop()
	for !condition() {
		select {
		case <-deadline.C:
			t.Fatal("admission state did not become ready")
		case <-time.After(time.Millisecond):
		}
	}
}

func TestCoreAdmissionConfigurationAndBudget(t *testing.T) {
	t.Setenv("ISUREAL_CORE_REGION_LIMITS", "")
	t.Setenv("ISUREAL_CORE_ADMISSION", "")
	if newCoreAdmission(admissionTestConfig()) != nil {
		t.Fatal("unrequested experiment enabled")
	}
	t.Setenv("ISUREAL_CORE_ADMISSION", "true")
	t.Setenv("ISUREAL_CORE_REGION_CONCURRENCY", "")
	for _, pool := range []string{"30", "45", "60"} {
		t.Run(pool, func(t *testing.T) {
			t.Setenv("ISUREAL_CORE_DB_MAX_OPEN", pool)
			a := newCoreAdmission(admissionTestConfig())
			total := 3 // control requests plus the batched notification worker
			for _, r := range a.regions {
				total += cap(r.slots)
			}
			if total > databaseConnectionLimit(admissionTestConfig()) || len(a.regions) != 3 {
				t.Fatal("request budget exceeds shared DB capacity")
			}
		})
	}
	for _, c := range []*Config{{Region: "asia"}, {Region: "europe", EdgeUpstream: "http://127.0.0.1:8081"}} {
		if newCoreAdmission(c) != nil {
			t.Fatal("single listener or overseas edge was gated")
		}
	}
	for _, value := range []string{"0", "10", "bad"} {
		t.Run("invalid-"+value, func(t *testing.T) {
			t.Setenv("ISUREAL_CORE_DB_MAX_OPEN", "30")
			t.Setenv("ISUREAL_CORE_REGION_CONCURRENCY", value)
			defer func() {
				if recover() == nil {
					t.Fatal("invalid concurrency was accepted")
				}
			}()
			newCoreAdmission(admissionTestConfig())
		})
	}
}

func TestCoreAdmissionRegionLimits(t *testing.T) {
	t.Setenv("ISUREAL_CORE_ADMISSION", "true")
	for _, tc := range []struct {
		name, pool, listeners, limits, concurrency string
		want                                       map[string]int
	}{
		{name: "default30", pool: "30", want: map[string]int{"asia": 9, "europe": 9, "america": 9}},
		{name: "default45", pool: "45", want: map[string]int{"asia": 14, "europe": 14, "america": 14}},
		{name: "default60", pool: "60", want: map[string]int{"asia": 19, "europe": 19, "america": 19}},
		{name: "legacy", concurrency: "2", want: map[string]int{"asia": 2, "europe": 2, "america": 2}},
		{name: "weighted", limits: "asia:5,europe:11,america:11", want: map[string]int{"asia": 5, "europe": 11, "america": 11}},
		{name: "reordered-under-budget", limits: "america:3,asia:1,europe:2", want: map[string]int{"asia": 1, "europe": 2, "america": 3}},
		{name: "larger-pool", pool: "45", limits: "asia:2,europe:20,america:20", want: map[string]int{"asia": 2, "europe": 20, "america": 20}},
		{name: "two-listeners", listeners: "asia:8080,europe:8081", limits: "europe:26,asia:1", want: map[string]int{"asia": 1, "europe": 26}},
		{name: "missing", limits: "asia:5,europe:11"},
		{name: "unknown", limits: "asia:5,europe:10,america:11,other:1"},
		{name: "non-listener", listeners: "asia:8080,europe:8081", limits: "asia:1,europe:1,america:1"},
		{name: "duplicate", limits: "asia:5,europe:11,asia:11"},
		{name: "duplicate-extra", limits: "asia:1,europe:1,america:1,asia:1"},
		{name: "zero", limits: "asia:0,europe:11,america:11"},
		{name: "negative", limits: "asia:-1,europe:11,america:11"},
		{name: "over-budget", limits: "asia:6,europe:11,america:11"},
		{name: "integer-overflow", limits: "asia:9223372036854775807,europe:1,america:1"},
		{name: "parse-overflow", limits: "asia:9999999999999999999999999,europe:1,america:1"},
		{name: "not-integer", limits: "asia:bad,europe:11,america:11"},
		{name: "fraction", limits: "asia:1.5,europe:11,america:11"},
		{name: "empty-limit", limits: "asia:,europe:11,america:11"},
		{name: "malformed", limits: "asia=5,europe:11,america:11"},
		{name: "extra-colon", limits: "asia:5:1,europe:11,america:11"},
		{name: "empty-entry", limits: "asia:5,europe:11,america:11,"},
		{name: "both-settings", limits: "asia:5,europe:11,america:11", concurrency: "1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("ISUREAL_CORE_DB_MAX_OPEN", tc.pool)
			t.Setenv("ISUREAL_CORE_REGION_LIMITS", tc.limits)
			t.Setenv("ISUREAL_CORE_REGION_CONCURRENCY", tc.concurrency)
			config := admissionTestConfig()
			if tc.listeners != "" {
				config.ListenRegions = tc.listeners
			}
			if tc.want == nil {
				defer func() {
					if p := recover(); p == nil || !strings.Contains(fmt.Sprint(p), "ISUREAL_CORE_REGION_LIMITS") {
						t.Fatalf("expected region limits configuration panic, got %v", p)
					}
				}()
				newCoreAdmission(config)
				return
			}
			a := newCoreAdmission(config)
			if len(a.regions) != len(tc.want) || a.stats()["control"].Limit != 2 {
				t.Fatal("listener regions or control budget changed")
			}
			for region, want := range tc.want {
				if got := a.stats()[region].Limit; got != want {
					t.Fatalf("%s limit=%d, want %d", region, got, want)
				}
			}
		})
	}
}

func TestCoreAdmissionRegionLimitsIgnoredOutsideSharedAdmission(t *testing.T) {
	t.Setenv("ISUREAL_CORE_REGION_LIMITS", "invalid")
	t.Setenv("ISUREAL_CORE_REGION_CONCURRENCY", "1")
	for _, enabled := range []string{"", "false", "TRUE"} {
		t.Setenv("ISUREAL_CORE_ADMISSION", enabled)
		if newCoreAdmission(admissionTestConfig()) != nil {
			t.Fatal("disabled admission parsed region limits")
		}
	}
	t.Setenv("ISUREAL_CORE_ADMISSION", "true")
	for _, config := range []*Config{nil, {Region: "asia"}, {Region: "europe", EdgeUpstream: "http://127.0.0.1:8081"}} {
		if newCoreAdmission(config) != nil {
			t.Fatal("region limits affected a single listener or edge")
		}
	}
}

func TestCoreAdmissionCanceledQueueAndPanicRelease(t *testing.T) {
	r := &admissionRegion{slots: make(chan struct{}, 1)}
	first, err := r.acquire(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { _, err := r.acquire(ctx); done <- err }()
	waitAdmission(t, func() bool { return r.waiting.Load() == 1 })
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	first.release()
	first.release() // SSE release plus the middleware defer must be idempotent.
	if r.active.Load() != 0 || r.waiting.Load() != 0 || r.canceled.Load() != 1 || r.waitNanos.Load() == 0 {
		t.Fatal("cancellation leaked a slot or lost its wait counters")
	}
	a := &coreAdmission{regions: map[string]*admissionRegion{"asia": r}}
	e := echo.New()
	c := e.NewContext(httptest.NewRequest("GET", "/probe", nil), httptest.NewRecorder())
	func() {
		defer func() { _ = recover() }()
		_ = coreAdmissionMiddleware(a, "asia")(func(*echo.Context) error { panic("handler") })(c)
	}()
	if r.active.Load() != 0 || len(r.slots) != 0 {
		t.Fatal("handler panic leaked a slot")
	}
}

func TestCoreAdmissionStatsAndTraceQueuedWait(t *testing.T) {
	t.Setenv("ISUREAL_CORE_ADMISSION", "true")
	t.Setenv("ISUREAL_CORE_REGION_CONCURRENCY", "")
	t.Setenv("ISUREAL_CORE_REGION_LIMITS", "asia:1,europe:2,america:3")
	a := newCoreAdmission(admissionTestConfig())
	held, err := a.regions["asia"].acquire(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer held.release()
	exporter := testTraceExporter(t)
	e := echo.New()
	e.Use(TracingMiddleware(), coreAdmissionMiddleware(a, "asia"))
	e.GET("/probe", func(c *echo.Context) error { return c.NoContent(204) })
	done := make(chan struct{})
	go func() {
		defer close(done)
		e.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/probe", nil))
	}()
	waitAdmission(t, func() bool { return a.stats()["asia"].Waiting == 1 })
	db, err := sql.Open("pgx", "postgres://private-user:private-password@127.0.0.1:1/private-db")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	recorder := httptest.NewRecorder()
	diagnosticsHandler(db, a).ServeHTTP(recorder, httptest.NewRequest("GET", "/debug/stats", nil))
	var snapshot struct {
		Admission map[string]admissionStats `json:"core_admission"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot.Admission["asia"].Waiting != 1 || snapshot.Admission["asia"].InUse != 1 || strings.Contains(recorder.Body.String(), "private-") {
		t.Fatal("admission stats missing or exposed credentials")
	}
	for region, want := range map[string]int{"asia": 1, "europe": 2, "america": 3, "control": 2} {
		if got := snapshot.Admission[region].Limit; got != want {
			t.Fatalf("/debug/stats %s limit=%d, want %d", region, got, want)
		}
	}
	held.release()
	<-done
	for _, span := range exporter.GetSpans() {
		if span.Name == "core.admission.wait" && span.EndTime.After(span.StartTime) && span.Parent.SpanID().IsValid() {
			return
		}
	}
	t.Fatal("queued wait was not traced separately from SQL")
}

// Exercise real sql.DB capacity: queued asia requests must not acquire it while
// europe, america and the control path can still make progress.
func TestCoreAdmissionProtectsOtherRegionsAndControl(t *testing.T) {
	for _, limits := range []string{"", "asia:1,europe:2,america:3"} {
		t.Run("limits="+limits, func(t *testing.T) {
			t.Setenv("ISUREAL_CORE_REGION_LIMITS", limits)
			t.Setenv("ISUREAL_CORE_REGION_CONCURRENCY", "")
			if limits == "" {
				t.Setenv("ISUREAL_CORE_REGION_CONCURRENCY", "1")
			}
			testCoreAdmissionProtectsOtherRegionsAndControl(t)
		})
	}
}

func testCoreAdmissionProtectsOtherRegionsAndControl(t *testing.T) {
	t.Helper()
	db, _ := architectureDB(t)
	db.SetMaxOpenConns(4)
	t.Setenv("ISUREAL_CORE_ADMISSION", "true")
	a := newCoreAdmission(admissionTestConfig())
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	blocked := make(chan struct{})
	release := make(chan struct{})
	var releaseOnce sync.Once
	defer releaseOnce.Do(func() { close(release) })
	servers := map[string]*echo.Echo{}
	for _, region := range []string{"asia", "europe", "america"} {
		e := echo.New()
		e.Use(coreAdmissionMiddleware(a, region))
		e.GET("/probe", func(c *echo.Context) error {
			conn, err := db.Conn(c.Request().Context())
			if err != nil {
				return err
			}
			defer conn.Close()
			if c.QueryParam("hold") == "true" {
				close(blocked)
				<-release
			}
			var one int
			if err := conn.QueryRowContext(c.Request().Context(), "SELECT 1").Scan(&one); err != nil {
				return err
			}
			return c.NoContent(204)
		})
		e.POST("/api/admin/posting-window", func(c *echo.Context) error {
			var one int
			if err := db.GetContext(c.Request().Context(), &one, "SELECT 1"); err != nil {
				return err
			}
			return c.NoContent(204)
		})
		servers[region] = e
	}
	heldDone := make(chan struct{})
	go func() {
		defer close(heldDone)
		servers["asia"].ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/probe?hold=true", nil))
	}()
	<-blocked
	var queued sync.WaitGroup
	for range 12 {
		queued.Go(func() {
			servers["asia"].ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/probe", nil).WithContext(ctx))
		})
	}
	waitAdmission(t, func() bool { return a.regions["asia"].waiting.Load() == 12 })
	before := db.Stats()
	for _, region := range []string{"europe", "america"} {
		recorder := httptest.NewRecorder()
		servers[region].ServeHTTP(recorder, httptest.NewRequest("GET", "/probe", nil))
		if recorder.Code != 204 {
			t.Fatalf("%s could not progress: %d", region, recorder.Code)
		}
	}
	recorder := httptest.NewRecorder()
	servers["asia"].ServeHTTP(recorder, httptest.NewRequest("POST", "/api/admin/posting-window", nil))
	if recorder.Code != 204 || db.Stats().WaitCount != before.WaitCount || db.Stats().InUse != 1 {
		t.Fatal("queued asia requests consumed DB capacity or blocked control")
	}
	cancel()
	queued.Wait()
	releaseOnce.Do(func() { close(release) })
	<-heldDone
	if a.stats()["asia"].InUse != 0 || a.stats()["asia"].Waiting != 0 {
		t.Fatal("request slots were retained after completion")
	}
}

func TestCoreAdmissionDoesNotPinSlotsForSSE(t *testing.T) {
	db, _ := architectureDB(t)
	_, token := fixtureOwner(t, db)
	t.Setenv("ISUREAL_CORE_ADMISSION", "true")
	t.Setenv("ISUREAL_CORE_REGION_LIMITS", "")
	t.Setenv("ISUREAL_CORE_REGION_CONCURRENCY", "1")
	config := admissionTestConfig()
	config.Region = "europe" // fixtureOwner belongs to europe.
	r := &Runtime{DB: db, IPBans: &IPBanList{}, Images: NewImageCache(imageCacheMaxBytes), Notifications: newNotificationPoller(db), Admission: newCoreAdmission(config)}
	defer r.Notifications.close()
	server := httptest.NewServer(NewRegionServer(config, r))
	defer server.Close()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	for range 2 {
		req, _ := http.NewRequestWithContext(ctx, "GET", server.URL+"/api/notifications", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 || r.Admission.stats()["europe"].InUse != 0 {
			t.Fatalf("SSE status=%d, slots=%+v", resp.StatusCode, r.Admission.stats())
		}
	}
	for _, auth := range []bool{false, true} {
		req, _ := http.NewRequestWithContext(ctx, "GET", server.URL+"/api/me", nil)
		if auth {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		want := 401
		if auth {
			want = 200
		}
		if resp.StatusCode != want {
			t.Fatalf("admission changed authentication: %d, want %d", resp.StatusCode, want)
		}
	}
}

func BenchmarkCoreAdmissionMiddleware(b *testing.B) {
	for _, enabled := range []bool{false, true} {
		name := "off"
		var a *coreAdmission
		if enabled {
			name = "on"
			a = &coreAdmission{regions: map[string]*admissionRegion{"asia": {slots: make(chan struct{}, 9)}}}
		}
		b.Run(name, func(b *testing.B) {
			e := echo.New()
			req := httptest.NewRequest("GET", "/probe", nil)
			c := e.NewContext(req, httptest.NewRecorder())
			handler := coreAdmissionMiddleware(a, "asia")(func(*echo.Context) error { return nil })
			b.ReportAllocs()
			for range b.N {
				c.SetRequest(req)
				if err := handler(c); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
