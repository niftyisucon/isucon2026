package isureal

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"sort"
	"testing"

	"github.com/jmoiron/sqlx"
)

// Exercise the exact SQL predicate against all combinations of relationships
// and visibility; compare with the pre-existing Go authorization/feed filters.
func TestRankedFeedSQLMatchesGo(t *testing.T) {
	dsn := os.Getenv("ISUCON_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set ISUCON_TEST_DATABASE_URL to a disposable PostgreSQL database")
	}
	db, err := sqlx.Connect("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	tx, err := db.Beginx()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	_, err = tx.Exec(`CREATE TEMP TABLE ranked_candidates (id text, author_id text, visibility text)`)
	if err != nil {
		t.Fatal(err)
	}
	viewer := viewerContext{User: User{ID: "self"}, HasPostedToday: true, Friends: map[string]bool{}, FriendsOfFriends: map[string]bool{}, Blocked: map[string]bool{}}
	var posts []Post
	var want []string
	for mask := 0; mask < 8; mask++ {
		author := fmt.Sprintf("author-%d", mask)
		viewer.Friends[author] = mask&1 != 0
		viewer.FriendsOfFriends[author] = mask&2 != 0
		viewer.Blocked[author] = mask&4 != 0
	}
	// More than 51 eligible posts ensures SQL never truncates before scoring.
	for n := 0; n < 60; n++ {
		for mask := 0; mask < 8; mask++ {
			for _, visibility := range []string{"public", "friends", "friends_of_friends", "invalid"} {
				post := Post{ID: fmt.Sprintf("%03d-%d-%s", n, mask, visibility), AuthorID: fmt.Sprintf("author-%d", mask), Visibility: visibility}
				posts = append(posts, post)
			}
		}
	}
	viewer.FriendsOfFriends["self"] = true
	posts = append(posts, Post{ID: "self", AuthorID: "self", Visibility: "public"})
	for _, p := range posts {
		if _, err := tx.Exec(`INSERT INTO ranked_candidates VALUES ($1,$2,$3)`, p.ID, p.AuthorID, p.Visibility); err != nil {
			t.Fatal(err)
		}
		if p.AuthorID != viewer.ID && viewer.canView(p) && matchesFeed(viewer, nil, p, "friends-of-friends") {
			want = append(want, p.ID)
		}
	}
	sort.Strings(want)
	var got []string
	// The first three arguments mirror the handler's shared conditions.
	err = tx.SelectContext(context.Background(), &got, `SELECT p.id FROM ranked_candidates p WHERE p.author_id <> $1 AND ($2::timestamptz IS NULL OR p.id < $3)`+rankedFeedConditions+` ORDER BY p.id`, viewer.ID, nil, "", mapKeys(viewer.Blocked), mapKeys(viewer.Friends), mapKeys(viewer.FriendsOfFriends))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SQL returned %d candidates; Go expected %d", len(got), len(want))
	}
	t.Logf("%d candidates reduced to %d; SQL matches Go, without pre-score LIMIT", len(posts), len(got))
	// Empty relationship arrays must produce an empty result, not NULL/errors.
	got = nil
	err = tx.SelectContext(context.Background(), &got, `SELECT p.id FROM ranked_candidates p WHERE p.author_id <> $1 AND ($2::timestamptz IS NULL OR p.id < $3)`+rankedFeedConditions, viewer.ID, nil, "", []string{}, []string{}, []string{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatal("empty FoF returned candidates")
	}
}
