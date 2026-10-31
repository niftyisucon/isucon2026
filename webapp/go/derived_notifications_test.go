package isureal

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v5"
)

func notificationBoundary(t *testing.T, db *sqlx.DB) (time.Time, string) {
	t.Helper()
	var since time.Time
	var epoch string
	if err := db.QueryRow(`SELECT clock_timestamp(),generation::text FROM isureal_runtime_state WHERE id=true`).Scan(&since, &epoch); err != nil {
		t.Fatal(err)
	}
	return since, epoch
}

func pollNotificationOnce(t *testing.T, db *sqlx.DB, user, region string, since time.Time, epoch string) (*getNotificationsResponse, bool) {
	t.Helper()
	p := newNotificationPoller(db)
	defer p.close()
	p.epochSchema = true
	s := &notificationSubscription{UserID: user, Since: since, region: region, Epoch: epoch, result: make(chan *getNotificationsResponse, 1)}
	p.subscriptions[s] = struct{}{}
	data, err := json.Marshal([]*notificationSubscription{s})
	if err != nil {
		t.Fatal(err)
	}
	var rows []notificationPollRow
	if err := db.SelectContext(t.Context(), &rows, p.query(), string(data)); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("通知の確認結果が1行ではない: %d", len(rows))
	}
	p.deliver([]*notificationSubscription{s}, rows, nil)
	select {
	case n := <-s.result:
		return n, true
	default:
		return nil, false
	}
}

func openNotificationWindow(t *testing.T, e *echo.Echo, region, date string) openPostingWindowResponse {
	t.Helper()
	rec := windowLockResult(t, windowLockRequest(t, e, "POST", "/windows", "", fmt.Sprintf(`{"region":%q,"localDate":%q}`, region, date)), 201)
	var w openPostingWindowResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &w); err != nil {
		t.Fatal(err)
	}
	return w
}

func TestDerivedNotificationsFlag(t *testing.T) {
	for _, value := range []string{"", "true", "false"} {
		t.Run(value, func(t *testing.T) {
			t.Setenv("ISUREAL_DERIVED_NOTIFICATIONS", value)
			p := newNotificationPoller(nil)
			defer p.close()
			if p.derived != (value != "false") {
				t.Fatal("通知方式の既定値または切り替えが不正")
			}
			if value == "false" && p.query() != notificationPollQuery {
				t.Fatal("falseで旧SQLに戻らない")
			}
		})
	}
}

func TestNotificationModesSameScenarios(t *testing.T) {
	for _, mode := range []string{"false", "true"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("ISUREAL_DERIVED_NOTIFICATIONS", mode)
			for _, scenario := range []string{
				"開始後", "接続前開始", "時刻境界一致", "終了後", "退会", "存在しない利用者",
				"開始前リージョン変更", "開始後リージョン変更", "開始後リージョン往復",
				"開始時だけ別リージョン", "別リージョンの窓を挟む往復", "登録直後の開始", "開始後の登録",
			} {
				t.Run(scenario, func(t *testing.T) {
					db, _ := architectureDB(t)
					e := windowLockApp(db)
					mustSQL(t, db, `INSERT INTO users(id,username,display_name,secret_key_hash,region) VALUES('actor','actor','Actor',repeat('ab',32),'asia')`)
					user := "actor"
					since, epoch := notificationBoundary(t, db)
					want, wantRegion := "通知", "asia"
					move := func(region string) {
						windowLockResult(t, windowLockRequest(t, e, "PATCH", "/me", user, fmt.Sprintf(`{"region":%q}`, region)), 200)
					}
					register := func() {
						rec := windowLockResult(t, windowLockRequest(t, e, "POST", "/register", "", `{"username":"new_user","displayName":"New","region":"asia"}`), 201)
						var registered registerResponse
						if err := json.Unmarshal(rec.Body.Bytes(), &registered); err != nil {
							t.Fatal(err)
						}
						user = registered.User.ID
					}
					if scenario == "登録直後の開始" {
						register()
						since, epoch = notificationBoundary(t, db)
					}
					if scenario == "開始前リージョン変更" || scenario == "開始時だけ別リージョン" || scenario == "別リージョンの窓を挟む往復" {
						move("europe")
					}
					region := "asia"
					if scenario == "別リージョンの窓を挟む往復" {
						region = "europe"
					}
					w := openNotificationWindow(t, e, region, "2026-10-31")
					switch scenario {
					case "接続前開始":
						since, epoch = notificationBoundary(t, db)
						want = "待機"
					case "時刻境界一致":
						since, want = w.StartsAt, "待機"
					case "終了後":
						mustSQL(t, db, `UPDATE posting_windows SET ended_at=clock_timestamp() WHERE id=$1`, w.ID)
						want = "待機"
					case "退会":
						windowLockResult(t, windowLockRequest(t, e, "DELETE", "/me", user, ""), 204)
						want = "切断"
					case "存在しない利用者":
						user, want = "missing", "切断"
					case "開始前リージョン変更":
						want = "切断"
					case "開始後リージョン変更":
						move("europe")
						want = "切断"
					case "開始後リージョン往復":
						move("europe")
						move("asia")
					case "開始時だけ別リージョン":
						move("asia")
						if mode == "false" {
							want = "待機"
						}
					case "別リージョンの窓を挟む往復":
						move("asia")
						wantRegion = "europe"
						if mode == "true" {
							want = "待機"
						}
					case "開始後の登録":
						register()
						since, epoch = notificationBoundary(t, db)
						want = "待機"
					}
					n, received := pollNotificationOnce(t, db, user, "asia", since, epoch)
					if want == "待機" {
						if received {
							t.Fatalf("待機するはずが配信された: %+v", n)
						}
					} else if want == "切断" {
						if !received || n != nil {
							t.Fatalf("切断されない: %+v, %v", n, received)
						}
					} else {
						if !received || n == nil || n.PostingWindowID != w.ID || n.Region != wantRegion || !n.CreatedAt.Equal(w.StartsAt) {
							t.Fatalf("通知内容が異なる: %+v, %v", n, received)
						}
						if _, err := uuid.Parse(n.ID); err != nil || len(n.ID) != 36 {
							t.Fatalf("通知IDがUUID形式ではない: %q", n.ID)
						}
						// 同じ窓を再確認しても、ポーラー再作成後も同じIDを返す。
						again, _ := pollNotificationOnce(t, db, user, "asia", since, epoch)
						if again == nil || *again != *n {
							t.Fatalf("再確認で通知が変わった: %+v, %+v", n, again)
						}
						// 実際の再接続は新しいsinceになるため、既に開始した窓を再配信しない。
						freshSince, freshEpoch := notificationBoundary(t, db)
						if fresh, ok := pollNotificationOnce(t, db, user, "asia", freshSince, freshEpoch); ok {
							t.Fatalf("再接続で過去の通知が配信された: %+v", fresh)
						}
					}
					var count int
					if err := db.Get(&count, `SELECT count(*) FROM notifications WHERE posting_window_id=$1`, w.ID); err != nil {
						t.Fatal(err)
					}
					if mode == "true" && count != 0 {
						t.Fatalf("新方式で通知行が作成された: %d", count)
					}
				})
			}
		})
	}
}

func TestNotificationModesInitialize(t *testing.T) {
	for _, mode := range []string{"false", "true"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("ISUREAL_DERIVED_NOTIFICATIONS", mode)
			db, _ := architectureDB(t)
			mustSQL(t, db, `INSERT INTO users(id,username,display_name,secret_key_hash,region,created_at) VALUES('actor','actor','Actor',repeat('ab',32),'asia','2025-01-01');
 INSERT INTO posting_windows(id,region,local_date,starts_at,on_time_deadline,created_at) VALUES('initial','asia','2025-01-01','2025-01-01','2025-01-02','2025-01-01');
 INSERT INTO notifications VALUES('initial-notification','actor','initial','2025-01-01')`)
			execSQLFile(t, db, "003-initial-snapshot.sql")
			since, epoch := notificationBoundary(t, db)
			e := windowLockApp(db)
			openNotificationWindow(t, e, "asia", "2026-10-30")
			execSQLFile(t, db, "reset.sql")
			if n, ok := pollNotificationOnce(t, db, "actor", "asia", since, epoch); !ok || n != nil {
				t.Fatalf("初期化前の購読が切断されない: %+v", n)
			}
			var id string
			if err := db.Get(&id, `SELECT id FROM notifications`); err != nil || id != "initial-notification" {
				t.Fatalf("初期通知が保持されない: %q, %v", id, err)
			}
			since, epoch = notificationBoundary(t, db)
			if n, ok := pollNotificationOnce(t, db, "actor", "asia", since, epoch); ok {
				t.Fatalf("初期化後に過去の通知が配信された: %+v", n)
			}
			w := openNotificationWindow(t, e, "asia", "2026-10-31")
			n, ok := pollNotificationOnce(t, db, "actor", "asia", since, epoch)
			if !ok || n == nil || n.PostingWindowID != w.ID {
				t.Fatalf("初期化後に新しい通知が届かない: %+v", n)
			}
		})
	}
}

func TestNotificationModesWindowCommitWake(t *testing.T) {
	for _, mode := range []string{"false", "true"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("ISUREAL_DERIVED_NOTIFICATIONS", mode)
			db, dsn := architectureDB(t)
			mustSQL(t, db, `INSERT INTO users(id,username,display_name,secret_key_hash,region) VALUES('actor','actor','Actor',repeat('ab',32),'asia')`)
			listener, err := pgx.Connect(t.Context(), dsn)
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close(context.Background())
			if _, err := listener.Exec(t.Context(), `LISTEN isureal_state`); err != nil {
				t.Fatal(err)
			}
			// server.goと同じ件数条件を確認する。DDLは追加しない。
			var ready bool
			if err := db.Get(&ready, `SELECT count(*)=3 FROM pg_trigger WHERE tgname IN ('isureal_notification_changed','isureal_window_changed','isureal_user_changed') AND tgrelid IN ('notifications'::regclass,'posting_windows'::regclass,'users'::regclass) AND tgenabled='O'`); err != nil || !ready {
				t.Fatalf("起動時のトリガー検出に失敗: %v, %v", ready, err)
			}
			e := windowLockApp(db)
			for day := 1; day <= 2; day++ {
				since, epoch := notificationBoundary(t, db)
				w := openNotificationWindow(t, e, "asia", fmt.Sprintf("2026-10-%02d", day))
				ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
				n, err := listener.WaitForNotification(ctx)
				cancel()
				if err != nil || n.Payload != "changed" {
					t.Fatalf("窓開始でLISTENが起床しない（初回は0行UPDATE）: %+v, %v", n, err)
				}
				result, ok := pollNotificationOnce(t, db, "actor", "asia", since, epoch)
				if !ok || result == nil || result.PostingWindowID != w.ID {
					t.Fatalf("窓開始後の確認に失敗: %+v", result)
				}
			}
		})
	}
}

func TestNotificationModesStart5000Users(t *testing.T) {
	for _, mode := range []string{"false", "true"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("ISUREAL_DERIVED_NOTIFICATIONS", mode)
			db, _ := architectureDB(t)
			mustSQL(t, db, `INSERT INTO users(id,username,display_name,secret_key_hash,region) SELECT 'user-'||n,'user-'||n,'User',repeat('ab',32),'asia' FROM generate_series(1,5000) n`)
			e := windowLockApp(db)
			var durations []time.Duration
			for day := 1; day <= 5; day++ {
				start := time.Now()
				openNotificationWindow(t, e, "asia", fmt.Sprintf("2026-10-%02d", day))
				durations = append(durations, time.Since(start))
			}
			var count int
			if err := db.Get(&count, `SELECT count(*) FROM notifications`); err != nil {
				t.Fatal(err)
			}
			want := 0
			if mode == "false" {
				want = 25000
			}
			if count != want {
				t.Fatalf("通知行数=%d, 期待値=%d", count, want)
			}
			sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
			t.Logf("5000人、窓開始5回: 中央値=%s 最小=%s 最大=%s 通知行=%d", durations[2], durations[0], durations[4], count)
		})
	}
}

// 応答は従来どおり4フィールドで、内部の導出方式は公開しない。
func TestDerivedNotificationResponseFields(t *testing.T) {
	region, window := "asia", uuid.NewString()
	now := time.Now().UTC()
	t.Setenv("ISUREAL_DERIVED_NOTIFICATIONS", "true")
	p := newNotificationPoller(nil)
	defer p.close()
	snapshot := make([]*notificationSubscription, 0, 2)
	for _, user := range []string{"a", "b"} {
		s := &notificationSubscription{UserID: user, region: region, result: make(chan *getNotificationsResponse, 1)}
		p.subscriptions[s] = struct{}{}
		snapshot = append(snapshot, s)
	}
	p.deliver(snapshot, []notificationPollRow{
		{Ordinal: 1, UserRegion: &region, PostingWindowID: &window, Region: &region, CreatedAt: &now},
		{Ordinal: 2, UserRegion: &region, PostingWindowID: &window, Region: &region, CreatedAt: &now},
	}, nil)
	a, b := <-snapshot[0].result, <-snapshot[1].result
	if a.ID == b.ID {
		t.Fatal("異なる利用者に同じ通知IDが生成された")
	}
	if id, err := uuid.Parse(a.ID); err != nil || len(a.ID) != 36 || id.Version() != 5 {
		t.Fatalf("通知IDがUUID v5形式ではない: %q", a.ID)
	}
	restarted := newNotificationPoller(nil)
	defer restarted.close()
	s := &notificationSubscription{UserID: "a", region: region, result: make(chan *getNotificationsResponse, 1)}
	restarted.subscriptions[s] = struct{}{}
	restarted.deliver([]*notificationSubscription{s}, []notificationPollRow{{Ordinal: 1, UserRegion: &region, PostingWindowID: &window, Region: &region, CreatedAt: &now}}, nil)
	if again := <-s.result; *again != *a {
		t.Fatalf("ポーラー再作成で通知が変わった: %+v, %+v", a, again)
	}
	otherWindow := uuid.NewString()
	s = &notificationSubscription{UserID: "a", region: region, result: make(chan *getNotificationsResponse, 1)}
	restarted.subscriptions[s] = struct{}{}
	restarted.deliver([]*notificationSubscription{s}, []notificationPollRow{{Ordinal: 1, UserRegion: &region, PostingWindowID: &otherWindow, Region: &region, CreatedAt: &now}}, nil)
	if other := <-s.result; other.ID == a.ID {
		t.Fatal("異なる窓に同じ通知IDが生成された")
	}
	var fields map[string]any
	data, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}
	if len(fields) != 4 || fields["id"] != a.ID || fields["postingWindowId"] != window || fields["region"] != region || fields["createdAt"] != now.Format(time.RFC3339Nano) {
		t.Fatalf("応答形式が変わった: %s", data)
	}
}

func TestNotificationModesInFlightWindow(t *testing.T) {
	for _, mode := range []string{"false", "true"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("ISUREAL_DERIVED_NOTIFICATIONS", mode)
			db, _ := architectureDB(t)
			mustSQL(t, db, `INSERT INTO users(id,username,display_name,secret_key_hash,region) VALUES('actor','actor','Actor',repeat('ab',32),'asia');
 CREATE FUNCTION pause_window() RETURNS trigger LANGUAGE plpgsql AS $$
 BEGIN PERFORM pg_advisory_xact_lock(20261031,7299); RETURN NEW; END $$;
 CREATE TRIGGER pause_window BEFORE INSERT ON posting_windows FOR EACH ROW EXECUTE FUNCTION pause_window()`)
			e := windowLockApp(db)
			before, epoch := notificationBoundary(t, db)
			gate := postingGate(t, db)
			defer gate.Rollback()
			opening := windowLockRequest(t, e, "POST", "/windows", "", `{"region":"asia","localDate":"2026-10-31"}`)
			waitPostingLock(t, db, 7299, 1)
			// starts_at取得後・コミット前の接続は、新旧ともこの窓を受信しない。
			during, _ := notificationBoundary(t, db)
			if n, ok := pollNotificationOnce(t, db, "actor", "asia", before, epoch); ok {
				t.Fatalf("未コミットの窓が配信された: %+v", n)
			}
			registration := windowLockRequest(t, e, "POST", "/register", "", `{"username":"late_user","displayName":"Late","region":"asia"}`)
			waitPostingLock(t, db, 7201, 1)
			if err := gate.Commit(); err != nil {
				t.Fatal(err)
			}
			windowLockResult(t, opening, 201)
			registered := windowLockResult(t, registration, 201)
			var user registerResponse
			if err := json.Unmarshal(registered.Body.Bytes(), &user); err != nil {
				t.Fatal(err)
			}
			if n, ok := pollNotificationOnce(t, db, "actor", "asia", before, epoch); !ok || n == nil {
				t.Fatalf("開始前の購読に通知が届かない: %+v", n)
			}
			if n, ok := pollNotificationOnce(t, db, "actor", "asia", during, epoch); ok {
				t.Fatalf("開始時刻より後の接続に通知が届いた: %+v", n)
			}
			after, _ := notificationBoundary(t, db)
			if n, ok := pollNotificationOnce(t, db, user.User.ID, "asia", after, epoch); ok {
				t.Fatalf("開始待ちだった登録者に過去の通知が届いた: %+v", n)
			}
		})
	}
}

func TestNotificationModesRolledBackWindow(t *testing.T) {
	for _, mode := range []string{"false", "true"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("ISUREAL_DERIVED_NOTIFICATIONS", mode)
			db, dsn := architectureDB(t)
			mustSQL(t, db, `INSERT INTO users(id,username,display_name,secret_key_hash,region) VALUES('actor','actor','Actor',repeat('ab',32),'asia')`)
			listener, err := pgx.Connect(t.Context(), dsn)
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close(context.Background())
			if _, err := listener.Exec(t.Context(), `LISTEN isureal_state`); err != nil {
				t.Fatal(err)
			}
			since, epoch := notificationBoundary(t, db)
			tx, err := db.Beginx()
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback()
			if _, err := tx.Exec(`UPDATE posting_windows SET ended_at=clock_timestamp() WHERE region='asia' AND ended_at IS NULL;
 INSERT INTO posting_windows(id,region,local_date,starts_at,on_time_deadline) VALUES('rolled-back','asia','2026-10-31',clock_timestamp(),clock_timestamp())`); err != nil {
				t.Fatal(err)
			}
			if mode == "false" {
				if _, err := tx.Exec(`INSERT INTO notifications VALUES('rolled-back','actor','rolled-back',clock_timestamp())`); err != nil {
					t.Fatal(err)
				}
			}
			if err := tx.Rollback(); err != nil {
				t.Fatal(err)
			}
			if n, ok := pollNotificationOnce(t, db, "actor", "asia", since, epoch); ok {
				t.Fatalf("ロールバックした窓が配信された: %+v", n)
			}
			ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
			defer cancel()
			if n, err := listener.WaitForNotification(ctx); err == nil {
				t.Fatalf("ロールバックした窓がLISTENを起床させた: %+v", n)
			}
		})
	}
}
