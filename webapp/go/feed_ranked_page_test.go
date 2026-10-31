package isureal

import (
	"context"
	"database/sql"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
)

// Preserve the pre-change full-detail path as an independent differential oracle.
func selectRankedPage(ctx context.Context, db *sqlx.Tx, viewer viewerContext, input getFeedRequest) (getFeedResponse, error) {
	page, err := selectRankedPageData(ctx, db, viewer, input)
	return page.response(), err
}

func TestRankedSigningReturnsConnectionBeforeFormatting(t *testing.T) {
	if !imageURLCacheEnabled {
		t.Skip("requires the signature cache lock to pause formatting")
	}
	db, _ := architectureDB(t)
	mustSQL(t, db, `INSERT INTO users(id,username,secret_key_hash,display_name,region)
 SELECT 'u'||n,'u'||n,repeat('ab',32),'User '||n,'asia' FROM generate_series(0,2) n`)
	mustSQL(t, db, `INSERT INTO posting_windows(id,region,local_date,starts_at,on_time_deadline)
 VALUES('w','asia',current_date,now(),now()+interval '1 hour')`)
	mustSQL(t, db, `INSERT INTO posts(id,author_id,posting_window_id,image_id,visibility,location_kind,posted_at)
	 VALUES('own','u0','w','own-image','public','none',now()),('target','u2','w','target-image','public','none',now())`)
	mustSQL(t, db, `INSERT INTO friendships(id,user_low_id,user_high_id) VALUES('direct','u0','u1'),('fof','u1','u2')`)
	db.SetMaxOpenConns(1)
	// Prevent the formatter from finishing, independently of how fast HMAC is.
	shard := &signedImageURLs.shards[0xab%imageURLCacheShards]
	shard.mu.Lock()
	locked := true
	defer func() {
		if locked {
			shard.mu.Unlock()
		}
	}()
	held, err := db.Conn(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer held.Close()
	before := db.Stats().WaitCount
	type result struct {
		response getFeedResponse
		err      error
	}
	done := make(chan result, 1)
	go func() {
		response, err := getRankedFeed(t.Context(), db, "u0", getFeedRequest{Feed: "friends-of-friends"})
		done <- result{response, err}
	}()
	waitAdmission(t, func() bool { return db.Stats().WaitCount > before })
	held.Close() // the queued feed request now owns the only connection
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("formatting retained the only DB connection: %v", err)
	}
	conn.Close()
	select {
	case r := <-done:
		t.Fatalf("expected to be paused in signing: %+v", r)
	default:
	}
	// A writer must also progress while signing is paused. The already-selected
	// page keeps its snapshot; the next request must observe this deletion.
	mustSQL(t, db, `UPDATE posts SET deleted_at=clock_timestamp() WHERE id='target'`)
	shard.mu.Unlock()
	locked = false
	r := <-done
	if r.err != nil || len(r.response.Items) != 1 || r.response.Items[0].ID != "target" {
		t.Fatalf("snapshot page changed during formatting: %+v", r)
	}
	next, err := getRankedFeed(t.Context(), db, "u0", getFeedRequest{Feed: "friends-of-friends"})
	if err != nil || len(next.Items) != 0 {
		t.Fatal("the next request failed to observe deletion", err)
	}
}

func originalRankedPage(ctx context.Context, db sqlx.QueryerContext, viewer viewerContext, input getFeedRequest) (getFeedResponse, error) {
	posts, err := selectPostsWithAuthor(ctx, db, `AND p.author_id<>$1 AND w.ended_at IS NULL
 AND ($2::timestamptz IS NULL OR (p.posted_at,p.id)<($2,$3))`+rankedFeedConditions+` ORDER BY p.posted_at DESC,p.id DESC`, viewer.ID, nil, "", mapKeys(viewer.Blocked), mapKeys(viewer.Friends), mapKeys(viewer.FriendsOfFriends))
	if err != nil {
		return getFeedResponse{}, err
	}
	candidates := []PostWithAuthor{}
	for _, p := range posts {
		if viewer.canView(p.Post) && matchesFeed(viewer, nil, p.Post, "friends-of-friends") {
			candidates = append(candidates, p)
		}
	}
	scores, err := recommendationScores(ctx, db, viewer, candidates)
	if err != nil {
		return getFeedResponse{}, err
	}
	sort.SliceStable(candidates, func(i, j int) bool { return scores[candidates[i].ID] > scores[candidates[j].ID] })
	response := getFeedResponse{Items: []feedItemResponse{}}
	for _, p := range candidates {
		score := scores[p.ID]
		if input.BeforeScore != nil && !rankedBefore(score, p.Post, *input.BeforeScore, *input.BeforePostedAt, input.BeforeID) {
			continue
		}
		response.Items = append(response.Items, feedItemResponse{createPostResponse: newPostResponse(p.Post, p.Author), Score: &score})
		if len(response.Items) == 51 {
			break
		}
	}
	response.HasMore = len(response.Items) > 50
	if response.HasMore {
		response.Items = response.Items[:50]
	}
	return response, nil
}

func TestRankedLateDetailsMatchOriginal(t *testing.T) {
	db, _ := architectureDB(t)
	mustSQL(t, db, `INSERT INTO users(id,username,secret_key_hash,display_name,bio,region,status)
 SELECT 'u'||n,'u'||n,repeat('ab',32),'User '||n,repeat('b',500),'asia',CASE WHEN n=6 THEN 'deleted' ELSE 'active' END FROM generate_series(0,602) n`)
	mustSQL(t, db, `INSERT INTO posting_windows(id,region,local_date,starts_at,on_time_deadline,ended_at) VALUES
 ('w','asia','2025-01-01','2025-01-01','2025-01-02',NULL),('old','asia','2024-12-31','2024-12-31','2025-01-01','2025-01-01')`)
	mustSQL(t, db, `INSERT INTO posts(id,author_id,posting_window_id,image_id,caption,visibility,location_kind,posted_at,deleted_at)
 SELECT 'p'||lpad(n::text,4,'0'),'u'||n,CASE WHEN n=8 THEN 'old' ELSE 'w' END,'image-'||n,repeat('caption',140),
 CASE n%3 WHEN 0 THEN 'public' WHEN 1 THEN 'friends_of_friends' ELSE 'friends' END,'none',
 '2025-01-01'::timestamptz+(n/3)*interval '1 second', CASE WHEN n=7 THEN now() END FROM generate_series(0,601) n`)
	mustSQL(t, db, `INSERT INTO friendships(id,user_low_id,user_high_id) VALUES('own','u0','u1')`)
	mustSQL(t, db, `INSERT INTO friendships(id,user_low_id,user_high_id) SELECT 'f'||n,'u1','u'||n FROM generate_series(2,601) n`)
	mustSQL(t, db, `INSERT INTO block_relations(id,blocker_id,blocked_id) VALUES('block','u9','u0')`)
	mustSQL(t, db, `INSERT INTO comments(id,post_id,author_id,body,created_at,deleted_at)
 SELECT 'c'||n,'p'||lpad(n::text,4,'0'),'u1','reaction','2025-01-01',CASE WHEN n%10=0 THEN now() END FROM generate_series(2,601) n WHERE n%2=0`)
	mustSQL(t, db, `INSERT INTO realmojis(id,post_id,reactor_id,image_id,reaction_type,created_at,replaced_at)
 SELECT 'r'||n,'p'||lpad(n::text,4,'0'),'u1','img','happy','2025-01-01',CASE WHEN n%10=0 THEN now() END FROM generate_series(2,601) n WHERE n%4=0`)
	tx, err := db.BeginTxx(t.Context(), &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	viewer, err := loadViewerContext(t.Context(), tx, "u0")
	if err != nil {
		t.Fatal(err)
	}
	stripExpiry := func(r getFeedResponse) getFeedResponse {
		for i := range r.Items {
			r.Items[i].ImageURL = strings.Split(r.Items[i].ImageURL, "?")[0]
		}
		return r
	}
	input := getFeedRequest{Feed: "friends-of-friends"}
	seen := map[string]bool{}
	for page := 0; page < 20; page++ {
		want, err := originalRankedPage(t.Context(), tx, viewer, input)
		if err != nil {
			t.Fatal(err)
		}
		got, err := selectRankedPage(t.Context(), tx, viewer, input)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(stripExpiry(got), stripExpiry(want)) {
			t.Fatalf("page %d differs from original", page)
		}
		for _, item := range got.Items {
			if seen[item.ID] {
				t.Fatal("duplicate across pages", item.ID)
			}
			seen[item.ID] = true
		}
		if !got.HasMore {
			break
		}
		last := got.Items[len(got.Items)-1]
		input.BeforeScore = last.Score
		input.BeforePostedAt = &last.PostedAt
		input.BeforeID = last.ID
	}
	if len(seen) <= 51 {
		t.Fatalf("only %d candidates tested", len(seen))
	}
	t.Logf("compared %d candidates across score/time/id cursor pages", len(seen))
	// A concurrent delete is not allowed to remove details from an established
	// ranking snapshot. A subsequent request must observe it immediately.
	before, err := selectRankedPage(t.Context(), tx, viewer, getFeedRequest{})
	if err != nil || len(before.Items) < 2 {
		t.Fatalf("before concurrent delete: %+v %v", before, err)
	}
	deletedAuthor, deletedPost := before.Items[0].Author.ID, before.Items[1].ID
	if _, err := db.ExecContext(t.Context(), `UPDATE users SET status='deleted' WHERE id=$1`, deletedAuthor); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(t.Context(), `UPDATE posts SET deleted_at=now() WHERE id=$1`, deletedPost); err != nil {
		t.Fatal(err)
	}
	got, err := selectRankedPage(t.Context(), tx, viewer, getFeedRequest{})
	if err != nil {
		t.Fatal(err)
	}
	want, err := originalRankedPage(t.Context(), tx, viewer, getFeedRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(stripExpiry(got), stripExpiry(want)) || !reflect.DeepEqual(stripExpiry(got), stripExpiry(before)) {
		t.Fatal("snapshot changed after concurrent writes")
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	// Reuse the same pooled connection so the final SHOW also verifies that
	// custom planning is reset when the ranked-feed transaction ends.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	actual, err := getRankedFeed(t.Context(), db, "u0", getFeedRequest{})
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range actual.Items {
		if item.Author.ID == deletedAuthor || item.ID == deletedPost {
			t.Fatal("new request returned deleted data")
		}
	}
	for _, optimized := range []bool{false, true} {
		start := time.Now()
		for range 5 {
			if optimized {
				_, err = getRankedFeed(t.Context(), db, "u0", getFeedRequest{})
			} else {
				viewer, err = loadViewerContext(t.Context(), db, "u0")
				if err != nil {
					t.Fatal(err)
				}
				_, err = originalRankedPage(t.Context(), db, viewer, getFeedRequest{})
			}
			if err != nil {
				t.Fatal(err)
			}
		}
		t.Logf("optimized=%v first-page mean=%s (both include viewer lookup; new also includes transaction)", optimized, time.Since(start)/5)
	}
	// Exercise the small-candidate path as well as the 397-candidate aggregate
	// path above, using the independent pre-change page/score implementation.
	mustSQL(t, db, `UPDATE posts SET deleted_at=now() WHERE id>'p0060'`)
	small, err := getRankedFeed(t.Context(), db, "u0", getFeedRequest{})
	if err != nil {
		t.Fatal(err)
	}
	wantSmall, err := originalRankedPage(t.Context(), db, viewer, getFeedRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(stripExpiry(small), stripExpiry(wantSmall)) || len(small.Items) >= scoreAggregateThreshold {
		t.Fatal("small-candidate page differs")
	}
	var mode string
	if err := db.Get(&mode, `SHOW plan_cache_mode`); err != nil || mode != "auto" {
		t.Fatalf("transaction setting leaked: %q %v", mode, err)
	}
	mustSQL(t, db, `UPDATE posts SET deleted_at=now() WHERE author_id='u0'`)
	empty, err := getRankedFeed(t.Context(), db, "u0", getFeedRequest{})
	if err != nil || len(empty.Items) != 0 || empty.HasMore {
		t.Fatalf("unposted: %+v %v", empty, err)
	}
}
