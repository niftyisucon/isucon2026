package isureal

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"runtime/metrics"
	"strconv"
	"time"
)

var processStarted = time.Now()

// This listener is optional and independent of all public/region routers.
// It exports counters and CPU stacks only, never cmdline, heap or request data.
func NewDiagnosticsServer(address string, db *sql.DB, admission ...*coreAdmission) (*http.Server, net.Listener, error) {
	host, _, err := net.SplitHostPort(address)
	if err != nil || !net.ParseIP(host).IsLoopback() {
		return nil, nil, fmt.Errorf("ISUREAL_DIAGNOSTICS_ADDR must use a literal loopback IP and port")
	}
	socket, err := net.Listen("tcp", address)
	if err != nil {
		return nil, nil, err
	}
	server := &http.Server{Handler: diagnosticsHandler(db, admission...), ReadHeaderTimeout: 5 * time.Second, WriteTimeout: 35 * time.Second, IdleTimeout: 30 * time.Second}
	return server, socket, nil
}

func diagnosticsHandler(db *sql.DB, admission ...*coreAdmission) http.Handler {
	var gate *coreAdmission
	if len(admission) != 0 {
		gate = admission[0]
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /debug/stats", func(w http.ResponseWriter, r *http.Request) {
		names := []string{
			"/cpu/classes/gc/total:cpu-seconds", "/cpu/classes/user:cpu-seconds",
			"/cpu/classes/total:cpu-seconds", "/cpu/classes/idle:cpu-seconds",
			"/gc/heap/allocs:bytes", "/gc/heap/live:bytes", "/gc/cycles/total:gc-cycles",
			"/sched/gomaxprocs:threads", "/sched/goroutines:goroutines",
			"/sync/mutex/wait/total:seconds",
		}
		samples := make([]metrics.Sample, len(names))
		for i, name := range names {
			samples[i].Name = name
		}
		metrics.Read(samples)
		values := make(map[string]any, len(samples))
		for _, sample := range samples {
			switch sample.Value.Kind() {
			case metrics.KindUint64:
				values[sample.Name] = sample.Value.Uint64()
			case metrics.KindFloat64:
				values[sample.Name] = sample.Value.Float64()
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(struct {
			Time           time.Time                 `json:"time"`
			PID            int                       `json:"pid"`
			UptimeSeconds  float64                   `json:"uptime_seconds"`
			DB             sql.DBStats               `json:"db"`
			Runtime        map[string]any            `json:"runtime"`
			ImageURLs      map[string]uint64         `json:"image_url_cache"`
			Sessions       map[string]uint64         `json:"session_cache"`
			JPEGValidation map[string]uint64         `json:"jpeg_validation_cache"`
			CoreAdmission  map[string]admissionStats `json:"core_admission,omitempty"`
		}{time.Now().UTC(), os.Getpid(), time.Since(processStarted).Seconds(), db.Stats(), values, signedImageURLs.stats(), sessionCacheStats(), validatedJPEGs.stats(), gate.stats()})
	})
	mux.HandleFunc("GET /debug/pprof/profile", func(w http.ResponseWriter, r *http.Request) {
		seconds := 30
		if raw := r.URL.Query().Get("seconds"); raw != "" {
			var err error
			seconds, err = strconv.Atoi(raw)
			if err != nil || seconds < 1 || seconds > 30 {
				http.Error(w, "seconds must be between 1 and 30", http.StatusBadRequest)
				return
			}
		}
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		pprof.Profile(w, r)
	})
	return mux
}
