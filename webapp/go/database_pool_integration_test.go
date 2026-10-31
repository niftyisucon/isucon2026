package isureal

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"
)

func TestDatabasePoolLimitsAndReuse(t *testing.T) {
	dsn := os.Getenv("ISUCON_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set ISUCON_TEST_DATABASE_URL to a disposable PostgreSQL database")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	configureDatabasePool(db)
	ctx := context.Background()
	var held []*sql.Conn
	pids := map[int]bool{}
	for i := 0; i < 12; i++ {
		conn, err := db.Conn(ctx)
		if err != nil {
			t.Fatal(err)
		}
		held = append(held, conn)
		var pid int
		if err = conn.QueryRowContext(ctx, `SELECT pg_backend_pid()`).Scan(&pid); err != nil {
			t.Fatal(err)
		}
		pids[pid] = true
	}
	for _, conn := range held {
		if err = conn.Close(); err != nil {
			t.Fatal(err)
		}
	}
	if got := db.Stats().Idle; got != maxIdleDBConnections {
		t.Fatalf("idle=%d, want %d", got, maxIdleDBConnections)
	}
	held = nil
	for i := 0; i < maxDBConnections; i++ {
		conn, err := db.Conn(ctx)
		if err != nil {
			t.Fatal(err)
		}
		held = append(held, conn)
		if i < maxIdleDBConnections {
			var pid int
			if err = conn.QueryRowContext(ctx, `SELECT pg_backend_pid()`).Scan(&pid); err != nil {
				t.Fatal(err)
			}
			if !pids[pid] {
				t.Fatalf("idle connection %d was not reused", pid)
			}
		}
	}
	defer func() {
		for _, conn := range held {
			conn.Close()
		}
	}()
	timed, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()
	if _, err = db.Conn(timed); err == nil {
		t.Fatal("connection limit was not enforced")
	}
	if db.Stats().WaitCount == 0 {
		t.Fatal("expected connection wait to be counted")
	}
}
