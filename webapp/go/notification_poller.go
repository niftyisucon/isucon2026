package isureal

import (
	"context"
	"encoding/json"
	"github.com/jackc/pgx/v5"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jmoiron/sqlx"
)

type notificationSubscription struct {
	UserID string    `json:"user_id"`
	Since  time.Time `json:"since"`
	region string
	Epoch  string `json:"-"`
	result chan *getNotificationsResponse
}

// One worker queries all live subscriptions in the same database snapshot.
// Delivery uses committed notification rows, including their user-specific IDs.
type notificationPoller struct {
	db            *sqlx.DB
	ctx           context.Context
	cancel        context.CancelFunc
	wake          chan struct{}
	listening     atomic.Bool
	onReset       func()
	epochSchema   bool
	mu            sync.Mutex
	subscriptions map[*notificationSubscription]struct{}
	running       bool
}

func newNotificationPoller(db *sqlx.DB) *notificationPoller {
	ctx, cancel := context.WithCancel(context.Background())
	return &notificationPoller{db: db, subscriptions: make(map[*notificationSubscription]struct{}), ctx: ctx, cancel: cancel, wake: make(chan struct{}, 1)}
}

func (p *notificationPoller) subscribe(userID, region string, since time.Time, epoch ...string) *notificationSubscription {
	s := &notificationSubscription{UserID: userID, Since: since, region: region, result: make(chan *getNotificationsResponse, 1)}
	if len(epoch) > 0 {
		s.Epoch = epoch[0]
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.subscriptions[s] = struct{}{}
	p.signal()
	if !p.running {
		p.running = true
		go p.run()
	}
	return s
}

func (p *notificationPoller) unsubscribe(s *notificationSubscription) {
	p.mu.Lock()
	delete(p.subscriptions, s)
	p.mu.Unlock()
}

func (p *notificationPoller) reset() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for s := range p.subscriptions {
		s.result <- nil
		delete(p.subscriptions, s)
	}
}

type notificationPollRow struct {
	Epoch           string     `db:"epoch"`
	Ordinal         int        `db:"ordinal"`
	UserRegion      *string    `db:"user_region"`
	ID              *string    `db:"id"`
	PostingWindowID *string    `db:"posting_window_id"`
	Region          *string    `db:"region"`
	CreatedAt       *time.Time `db:"created_at"`
}

const notificationPollQuery = `
 SELECT s.ordinal, u.region AS user_region, x.id, x.posting_window_id, x.region, x.created_at
 FROM ROWS FROM(jsonb_to_recordset($1::jsonb) AS (user_id text, since timestamptz))
 WITH ORDINALITY AS s(user_id, since, ordinal)
 LEFT JOIN users u ON u.id = s.user_id AND u.status = 'active'
 LEFT JOIN LATERAL (
   SELECT n.id, n.posting_window_id, w.region, n.created_at
   FROM notifications n JOIN posting_windows w ON w.id = n.posting_window_id
   WHERE n.user_id = u.id AND n.created_at > s.since AND w.ended_at IS NULL
   ORDER BY n.created_at, n.id LIMIT 1
 ) x ON true`

func (p *notificationPoller) run() {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		p.mu.Lock()
		snapshot := make([]*notificationSubscription, 0, len(p.subscriptions))
		for s := range p.subscriptions {
			snapshot = append(snapshot, s)
		}
		if len(snapshot) == 0 {
			p.running = false
			p.mu.Unlock()
			return
		}
		p.mu.Unlock()
		data, _ := json.Marshal(snapshot)
		ctx, cancel := context.WithTimeout(p.ctx, 5*time.Second)
		var rows []notificationPollRow
		err := p.db.SelectContext(ctx, &rows, p.query(), string(data))
		cancel()
		if err != nil && p.ctx.Err() == nil {
			slog.Error("failed to poll notifications", "error", err)
		}
		p.deliver(snapshot, rows, err)
		lastPoll := time.Now()
	wait:
		for {
			select {
			case <-p.ctx.Done():
				p.reset()
				return
			case <-p.wake:
				break wait
			case <-ticker.C:
				// LISTEN wakes immediately on commits. The periodic batch is only
				// a safety net; disconnected listeners retain the old 500ms bound.
				if !p.listening.Load() || time.Since(lastPoll) >= 5*time.Second {
					break wait
				}
			}
		}
	}
}

func (p *notificationPoller) deliver(snapshot []*notificationSubscription, rows []notificationPollRow, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err != nil {
		for _, s := range snapshot {
			if _, ok := p.subscriptions[s]; ok {
				s.result <- nil
				delete(p.subscriptions, s)
			}
		}
		return
	}
	for _, row := range rows {
		s := snapshot[row.Ordinal-1]
		if _, ok := p.subscriptions[s]; !ok {
			continue
		}
		if (s.Epoch != "" && s.Epoch != row.Epoch) || row.UserRegion == nil || *row.UserRegion != s.region {
			s.result <- nil
		} else if row.ID != nil {
			s.result <- &getNotificationsResponse{ID: *row.ID, PostingWindowID: *row.PostingWindowID, Region: *row.Region, CreatedAt: *row.CreatedAt}
		} else {
			continue
		}
		delete(p.subscriptions, s)
	}
}

func (p *notificationPoller) signal() {
	select {
	case p.wake <- struct{}{}:
	default:
	}
}
func (p *notificationPoller) close() { p.cancel(); p.reset() }

func (p *notificationPoller) startListener(dsn string) {
	go func() {
		for p.ctx.Err() == nil {
			ctx, cancel := context.WithTimeout(p.ctx, 5*time.Second)
			conn, err := pgx.Connect(ctx, dsn)
			if err == nil {
				_, err = conn.Exec(ctx, "LISTEN isureal_state")
			}
			cancel()
			if err == nil {
				p.listening.Store(true)
				p.signal()
				for p.ctx.Err() == nil {
					n, e := conn.WaitForNotification(p.ctx)
					if e != nil {
						break
					}
					if n.Payload == "reset" {
						if !p.epochSchema {
							p.reset()
						}
						if p.onReset != nil {
							p.onReset()
						}
					}
					p.signal()
				}
			}
			p.listening.Store(false)
			p.signal()
			if conn != nil {
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				_ = conn.Close(ctx)
				cancel()
			}
			select {
			case <-p.ctx.Done():
				return
			case <-time.After(time.Second):
			}
		}
	}()
}

func (p *notificationPoller) query() string {
	if !p.epochSchema {
		return notificationPollQuery
	}
	return strings.Replace(notificationPollQuery, "SELECT s.ordinal,", "SELECT (SELECT generation::text FROM isureal_runtime_state WHERE id=true) AS epoch, s.ordinal,", 1)
}
