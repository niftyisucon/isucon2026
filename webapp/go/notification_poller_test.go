package isureal

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
)

func TestNotificationBatchQueryLocal(t *testing.T) {
	dsn := os.Getenv("ISUCON_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("requires disposable local PostgreSQL")
	}
	db, err := sqlx.Connect("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	// Temporary tables isolate this test from any existing schema/data.
	_, err = db.Exec(`CREATE TEMP TABLE users(id text, region text, status text);
 CREATE TEMP TABLE posting_windows(id text, region text, ended_at timestamptz);
 CREATE TEMP TABLE notifications(id text, user_id text, posting_window_id text, created_at timestamptz);
 INSERT INTO users VALUES ('a','asia','active'),('b','europe','active'),('c','asia','deleted');
 INSERT INTO posting_windows VALUES ('w','asia',NULL),('old','asia',now());
 INSERT INTO notifications VALUES ('n','a','w','2026-01-02'),('past','a','w','2025-01-01'),('ended','b','old','2026-01-02');`)
	if err != nil {
		t.Fatal(err)
	}
	since := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	mk := func(id, region string) *notificationSubscription {
		return &notificationSubscription{UserID: id, Since: since, region: region, result: make(chan *getNotificationsResponse, 1)}
	}
	subs := []*notificationSubscription{mk("a", "asia"), mk("b", "europe"), mk("c", "asia"), mk("missing", "asia"), mk("a", "europe"), mk("a", "asia")}
	subs[5].Since = time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC) // strict > boundary
	data, _ := json.Marshal(subs)
	var rows []notificationPollRow
	if err = db.SelectContext(context.Background(), &rows, notificationPollQuery, string(data)); err != nil {
		t.Fatal(err)
	}
	if len(rows) != len(subs) {
		t.Fatalf("rows=%d", len(rows))
	}
	p := newNotificationPoller(db)
	for _, s := range subs {
		p.subscriptions[s] = struct{}{}
	}
	p.deliver(subs, rows, nil)
	for i, s := range subs {
		select {
		case n := <-s.result:
			if i == 0 {
				if n == nil || n.ID != "n" || n.Region != "asia" {
					t.Fatalf("notification=%+v", n)
				}
			} else if i == 1 || i == 5 || n != nil {
				t.Fatalf("unexpected delivery for %d: %+v", i, n)
			}
		default:
			if i != 1 && i != 5 {
				t.Fatalf("missing result %d", i)
			}
		}
	}
}

func TestNotificationResetDiscardsInflight(t *testing.T) {
	p := newNotificationPoller(nil)
	s := &notificationSubscription{region: "asia", result: make(chan *getNotificationsResponse, 1)}
	p.subscriptions[s] = struct{}{}
	p.reset()
	region, id, window := "asia", "n", "w"
	now := time.Now()
	p.deliver([]*notificationSubscription{s}, []notificationPollRow{{Ordinal: 1, UserRegion: &region, ID: &id, PostingWindowID: &window, Region: &region, CreatedAt: &now}}, nil)
	if n := <-s.result; n != nil {
		t.Fatal("reset delivered notification")
	}
	select {
	case <-s.result:
		t.Fatal("stale query delivered after reset")
	default:
	}
	if len(p.subscriptions) != 0 {
		t.Fatal("reset retained subscription")
	}
}
