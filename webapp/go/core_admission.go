package isureal

import (
	"context"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/labstack/echo/v5"
)

// Bound work before authentication, body decoding or acquiring a DB connection.
// Each listener has its own slots, so one region cannot occupy every core slot.
// Three DB connections remain available for two control requests and the poller.
// This is opt-in: real throughput must be compared before enabling it in contest.
type coreAdmission struct {
	regions map[string]*admissionRegion
	control *admissionRegion
}

type admissionRegion struct {
	slots     chan struct{}
	active    atomic.Int64
	waiting   atomic.Int64
	admitted  atomic.Uint64
	waitCount atomic.Uint64
	waitNanos atomic.Uint64
	canceled  atomic.Uint64
}

type admissionStats struct {
	Limit     int    `json:"limit"`
	InUse     int64  `json:"in_use"`
	Waiting   int64  `json:"waiting"`
	Admitted  uint64 `json:"admitted"`
	WaitCount uint64 `json:"wait_count"`
	WaitNanos uint64 `json:"wait_nanoseconds"`
	Canceled  uint64 `json:"canceled"`
}

func newCoreAdmission(config *Config) *coreAdmission {
	if config == nil || config.EdgeUpstream != "" || os.Getenv("ISUREAL_CORE_ADMISSION") != "true" {
		return nil
	}
	listeners, err := config.RegionListeners()
	if err != nil {
		panic(err)
	}
	if len(listeners) < 2 {
		return nil
	}
	limit := (databaseConnectionLimit(config) - 3) / len(listeners)
	if raw := os.Getenv("ISUREAL_CORE_REGION_CONCURRENCY"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > limit {
			panic("ISUREAL_CORE_REGION_CONCURRENCY must be between 1 and the per-region DB budget")
		}
		limit = n
	}
	a := &coreAdmission{regions: make(map[string]*admissionRegion), control: &admissionRegion{slots: make(chan struct{}, 2)}}
	for _, listener := range listeners {
		a.regions[listener.Region] = &admissionRegion{slots: make(chan struct{}, limit)}
	}
	return a
}

type admissionLease struct {
	once sync.Once
	r    *admissionRegion
}

func (l *admissionLease) release() {
	l.once.Do(func() {
		l.r.active.Add(-1)
		<-l.r.slots
	})
}

type admissionContextKey struct{}

// SSE retains no request slot while waiting for a future notification.
func releaseCoreAdmission(ctx context.Context) {
	if l, ok := ctx.Value(admissionContextKey{}).(*admissionLease); ok {
		l.release()
	}
}

func (r *admissionRegion) acquire(ctx context.Context) (*admissionLease, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	select {
	case r.slots <- struct{}{}:
	default:
		r.waiting.Add(1)
		r.waitCount.Add(1)
		start := time.Now()
		_, span := startApplicationPhase(ctx, "core.admission.wait")
		select {
		case r.slots <- struct{}{}:
		case <-ctx.Done():
			r.canceled.Add(1)
			r.waiting.Add(-1)
			r.waitNanos.Add(uint64(time.Since(start)))
			span.End()
			return nil, ctx.Err()
		}
		r.waiting.Add(-1)
		r.waitNanos.Add(uint64(time.Since(start)))
		span.End()
	}
	r.active.Add(1)
	l := &admissionLease{r: r}
	// Cancellation can race with a slot becoming available.
	if err := ctx.Err(); err != nil {
		l.release()
		r.canceled.Add(1)
		return nil, err
	}
	r.admitted.Add(1)
	return l, nil
}

func coreAdmissionMiddleware(a *coreAdmission, region string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		if a == nil {
			return next
		}
		return func(c *echo.Context) error {
			r := a.regions[region]
			if c.Path() == "/api/initialize" || strings.HasPrefix(c.Path(), "/api/admin/") {
				r = a.control
			}
			if r == nil {
				return next(c)
			}
			l, err := r.acquire(c.Request().Context())
			if err != nil {
				return err
			}
			defer l.release()
			c.SetRequest(c.Request().WithContext(context.WithValue(c.Request().Context(), admissionContextKey{}, l)))
			return next(c)
		}
	}
}

func (a *coreAdmission) stats() map[string]admissionStats {
	if a == nil {
		return nil
	}
	out := make(map[string]admissionStats, len(a.regions)+1)
	add := func(name string, r *admissionRegion) {
		out[name] = admissionStats{cap(r.slots), r.active.Load(), r.waiting.Load(), r.admitted.Load(), r.waitCount.Load(), r.waitNanos.Load(), r.canceled.Load()}
	}
	for name, r := range a.regions {
		add(name, r)
	}
	add("control", a.control)
	return out
}
