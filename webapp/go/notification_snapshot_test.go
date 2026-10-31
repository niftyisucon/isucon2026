package isureal

import (
	"encoding/json"
	"reflect"
	"sort"
	"testing"
	"time"
)

func TestNotificationSnapshotMatchesStoredRows(t *testing.T) {
	t.Setenv("ISUREAL_DERIVED_NOTIFICATIONS", "false")
	db, _ := architectureDB(t)
	mustSQL(t, db, `INSERT INTO users(id,username,display_name,secret_key_hash,region,status)
 SELECT 'u'||n,'u'||n,'User',repeat('ab',32),'asia',CASE WHEN n=10 THEN 'deleted' ELSE 'active' END FROM generate_series(1,10)n;
 INSERT INTO posting_windows(id,region,local_date,starts_at,on_time_deadline)
 SELECT r,r,'2026-10-31','2026-10-31 00:00:00 UTC','2026-11-01 00:00:00 UTC' FROM unnest(ARRAY['asia','europe','america'])r;
 INSERT INTO notifications(id,user_id,posting_window_id,created_at)
 SELECT md5(u.id||w.id)::uuid::text,u.id,w.id,w.starts_at FROM users u CROSS JOIN posting_windows w;
 UPDATE posting_windows w SET (notification_user_ids,notification_ids)=(
 SELECT array_agg(user_id ORDER BY user_id),array_agg(id ORDER BY user_id) FROM notifications WHERE posting_window_id=w.id)
 WHERE region<>'america'`)
	// Deliberately tied starts_at values and unrelated random IDs exercise the
	// original created_at, notification ID ordering across all three regions.
	var subs []*notificationSubscription
	for _, user := range []string{"u1", "u5", "u10", "missing", "u1"} {
		for _, stamp := range []string{"2026-10-30T00:00:00Z", "2026-10-31T00:00:00Z", "2026-11-01T00:00:00Z"} {
			since, _ := time.Parse(time.RFC3339, stamp)
			subs = append(subs, &notificationSubscription{UserID: user, Since: since})
		}
	}
	data, err := json.Marshal(subs)
	if err != nil {
		t.Fatal(err)
	}
	var expected []notificationPollRow
	if err := db.Select(&expected, notificationPollQuery, string(data)); err != nil {
		t.Fatal(err)
	}
	mustSQL(t, db, `DELETE FROM notifications WHERE posting_window_id<>'america'`)
	for _, mode := range []string{"false", "true"} {
		t.Setenv("ISUREAL_DERIVED_NOTIFICATIONS", mode)
		p := newNotificationPoller(db)
		var got []notificationPollRow
		err := db.Select(&got, p.query(), string(data))
		p.close()
		if err != nil {
			t.Fatal(err)
		}
		sort.Slice(got, func(i, j int) bool { return got[i].Ordinal < got[j].Ordinal })
		sort.Slice(expected, func(i, j int) bool { return expected[i].Ordinal < expected[j].Ordinal })
		if !reflect.DeepEqual(got, expected) {
			t.Fatalf("mode %s: mixed representation differs from stored-row oracle\ngot=%+v\nwant=%+v", mode, got, expected)
		}
	}
}

func TestNotificationSnapshotWriterRollback(t *testing.T) {
	t.Setenv("ISUREAL_DERIVED_NOTIFICATIONS", "true")
	db, _ := architectureDB(t)
	mustSQL(t, db, `INSERT INTO users(id,username,display_name,secret_key_hash,region) VALUES('actor','actor','Actor',repeat('ab',32),'asia')`)
	e := windowLockApp(db)
	since, epoch := notificationBoundary(t, db)
	w := openNotificationWindow(t, e, "asia", "2026-10-30")
	before, ok := pollNotificationOnce(t, db, "actor", "asia", since, epoch)
	if !ok || before == nil {
		t.Fatal("snapshot notification missing")
	}
	var storedID string
	if err := db.Get(&storedID, `SELECT notification_ids[1] FROM posting_windows WHERE id=$1`, w.ID); err != nil || storedID != before.ID {
		t.Fatalf("stored ID: %q %v", storedID, err)
	}
	// Disabling writes and recreating the poller must still read earlier snapshots.
	t.Setenv("ISUREAL_DERIVED_NOTIFICATIONS", "false")
	after, ok := pollNotificationOnce(t, db, "actor", "asia", since, epoch)
	if !ok || after == nil || *before != *after {
		t.Fatalf("writer rollback changed event: %+v %+v", before, after)
	}
	// Failed snapshot persistence rolls back the preceding window ending too.
	mustSQL(t, db, `CREATE FUNCTION reject_snapshot() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'snapshot test failure'; END $$;
 CREATE TRIGGER reject_snapshot BEFORE UPDATE OF notification_user_ids ON posting_windows FOR EACH ROW EXECUTE FUNCTION reject_snapshot()`)
	windowLockResult(t, windowLockRequest(t, e, "POST", "/windows", "", `{"region":"asia","localDate":"2026-10-31"}`), 500)
	after, ok = pollNotificationOnce(t, db, "actor", "asia", since, epoch)
	if !ok || after == nil || *before != *after {
		t.Fatalf("failed transaction changed active event: %+v %+v", before, after)
	}
	var count int
	if err := db.Get(&count, `SELECT count(*) FROM posting_windows`); err != nil || count != 1 {
		t.Fatalf("partial window persisted: %d %v", count, err)
	}
}

func TestNotificationSnapshotEmptyAudienceAndSchema(t *testing.T) {
	t.Setenv("ISUREAL_DERIVED_NOTIFICATIONS", "false")
	db, _ := architectureDB(t)
	mustSQL(t, db, `ALTER TABLE posting_windows DROP COLUMN notification_user_ids, DROP COLUMN notification_ids`)
	p := newNotificationPoller(db)
	if p.snapshotSchema || p.query() != notificationPollQuery {
		t.Fatal("old schema did not use legacy reader")
	}
	p.close()
	t.Setenv("ISUREAL_DERIVED_NOTIFICATIONS", "true")
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("snapshot writer enabled without its schema")
			}
		}()
		newNotificationPoller(db)
	}()
	execSQLFile(t, db, "notification-snapshots.sql")
	execSQLFile(t, db, "notification-snapshots.sql")
	w := openNotificationWindow(t, windowLockApp(db), "asia", "2026-10-31")
	var empty bool
	if err := db.Get(&empty, `SELECT notification_user_ids='{}'::text[] AND notification_ids='{}'::text[] FROM posting_windows WHERE id=$1`, w.ID); err != nil || !empty {
		t.Fatalf("empty audience not persisted: %v %v", empty, err)
	}
}
