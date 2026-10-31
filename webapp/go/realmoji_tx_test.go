package isureal

import (
	"context"
	"errors"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

func TestRealMojiAuthorizationLocal(t *testing.T) {
	dsn := os.Getenv("ISUCON_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set ISUCON_TEST_DATABASE_URL to a disposable PostgreSQL database")
	}
	admin, err := sqlx.Connect("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	name := "realmoji_" + uuid.NewString()[:8]
	admin.MustExec(`CREATE SCHEMA ` + name)
	defer admin.MustExec(`DROP SCHEMA ` + name + ` CASCADE`)
	u, _ := url.Parse(dsn)
	q := u.Query()
	q.Set("search_path", name)
	u.RawQuery = q.Encode()
	db, err := sqlx.Connect("pgx", u.String())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	schema, err := os.ReadFile("../sql/001-schema.sql")
	if err != nil {
		t.Fatal(err)
	}
	db.MustExec(string(schema))
	db.MustExec(`INSERT INTO users(id,username,secret_key_hash,display_name,region) SELECT id,id,repeat('a',64),id,'asia' FROM unnest(ARRAY['actor','author']) id`)
	db.MustExec(`INSERT INTO posting_windows(id,region,local_date,starts_at,on_time_deadline) VALUES('window','asia',CURRENT_DATE,now(),now())`)
	db.MustExec(`INSERT INTO posts(id,author_id,posting_window_id,image_id,visibility,location_kind,posted_at) VALUES('post','author','window','image','public','none',now()),('own','actor','window','image2','public','none',now())`)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	check := func(want bool) {
		t.Helper()
		tx, err := beginRealMojiTx(ctx, db, "actor", "post")
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback()
		got, err := canViewPost(ctx, tx, "actor", "post")
		if err != nil || got != want {
			t.Fatalf("allowed=%v err=%v want=%v", got, err, want)
		}
	}
	check(true)
	db.MustExec(`UPDATE posts SET visibility='friends' WHERE id='post'`)
	check(false)
	db.MustExec(`INSERT INTO friendships(id,user_low_id,user_high_id,source_request_id) VALUES('friend','actor','author','request')`)
	check(true)
	db.MustExec(`INSERT INTO block_relations(id,blocker_id,blocked_id) VALUES('block','author','actor')`)
	check(false)
	db.MustExec(`DELETE FROM block_relations`)
	// The author changes visibility while holding the same user lock used by
	// post updates. Authorization must see the committed value after waiting.
	writer := db.MustBegin()
	writer.MustExec(`SELECT id FROM users WHERE id='author' FOR NO KEY UPDATE`)
	writer.MustExec(`UPDATE posts SET visibility='unknown' WHERE id='post'`)
	result := make(chan error, 1)
	go func() {
		tx, err := beginRealMojiTx(ctx, db, "actor", "post")
		if err != nil {
			result <- err
			return
		}
		defer tx.Rollback()
		allowed, err := canViewPost(ctx, tx, "actor", "post")
		if allowed {
			err = errors.New("stale visibility accepted")
		}
		result <- err
	}()
	select {
	case err := <-result:
		t.Fatalf("did not wait for author lock: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	if err := writer.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := <-result; err != nil {
		t.Fatal(err)
	}
	db.MustExec(`UPDATE posts SET visibility='public' WHERE id='post'`)
	db.MustExec(`UPDATE posting_windows SET ended_at=now() WHERE id='window'`)
	check(false)
	db.MustExec(`UPDATE posting_windows SET ended_at=NULL WHERE id='window'`)
	db.MustExec(`UPDATE posts SET deleted_at=now() WHERE id='own'`)
	check(false)
	db.MustExec(`UPDATE posts SET deleted_at=NULL WHERE id='own'`)

	for _, id := range []string{"missing", "own"} {
		tx, err := beginRealMojiTx(ctx, db, "actor", id)
		if tx != nil {
			tx.Rollback()
		}
		var api *APIError
		if !errors.As(err, &api) || api.Status != 403 {
			t.Fatalf("%s: %v", id, err)
		}
	}
	db.MustExec(`UPDATE users SET status='deleted' WHERE id='author'`)
	tx, err := beginRealMojiTx(ctx, db, "actor", "post")
	if tx != nil {
		tx.Rollback()
	}
	var api *APIError
	if !errors.As(err, &api) || api.Status != 404 {
		t.Fatalf("deleted author: %v", err)
	}
}
