package isureal

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
)

type PostLocation struct {
	Kind      string   `json:"kind" db:"location_kind"`
	Latitude  *float64 `json:"latitude,omitempty" db:"latitude"`
	Longitude *float64 `json:"longitude,omitempty" db:"longitude"`
	Locality  *string  `json:"locality,omitempty" db:"locality"`
}

func (location PostLocation) valid() bool {
	switch location.Kind {
	case "none":
		return location.Latitude == nil && location.Longitude == nil && location.Locality == nil
	case "exact":
		return location.Latitude != nil && location.Longitude != nil && location.Locality == nil &&
			*location.Latitude >= -90 && *location.Latitude <= 90 && *location.Longitude >= -180 && *location.Longitude <= 180
	case "approximate":
		return location.Latitude == nil && location.Longitude == nil && location.Locality != nil && validUserText(*location.Locality, 1, 100)
	default:
		return false
	}
}

type Post struct {
	ID              string `db:"id"`
	AuthorID        string `db:"author_id"`
	PostingWindowID string `db:"posting_window_id"`
	ImageID         string `db:"image_id"`
	Caption         string `db:"caption"`
	Visibility      string `db:"visibility"`
	PostLocation
	PostedAt       time.Time  `db:"posted_at"`
	OnTimeDeadline time.Time  `db:"on_time_deadline"`
	WindowEndedAt  *time.Time `db:"window_ended_at"`
}

const postColumns = `p.id, p.author_id, p.posting_window_id, p.image_id, p.caption, p.visibility,
  p.location_kind, p.latitude, p.longitude, p.locality, p.posted_at,
  w.on_time_deadline, w.ended_at AS window_ended_at`

func getPost(ctx context.Context, db sqlx.QueryerContext, postID string) (Post, error) {
	var post Post
	err := sqlx.GetContext(ctx, db, &post, `
  SELECT `+postColumns+`
  FROM posts p JOIN posting_windows w ON w.id = p.posting_window_id
  WHERE p.id = $1 AND p.deleted_at IS NULL
 `, postID)
	return post, err
}

type PostWithAuthor struct {
	Post
	Author User `db:"author"`
}

func selectPostsWithAuthor(ctx context.Context, db sqlx.QueryerContext, conditions string, args ...any) ([]PostWithAuthor, error) {
	var posts []PostWithAuthor
	if err := sqlx.SelectContext(ctx, db, &posts, `
  SELECT `+postColumns+`, `+userColumnsAs("u", "author")+`
  FROM posts p
  JOIN posting_windows w ON w.id = p.posting_window_id
  JOIN users u ON u.id = p.author_id AND u.status = 'active'
  WHERE p.deleted_at IS NULL `+conditions, args...); err != nil {
		return nil, err
	}
	for i := range posts {
		if err := posts[i].Author.setImageKey(); err != nil {
			return nil, err
		}
	}
	return posts, nil
}

func getPostWithAuthor(ctx context.Context, db sqlx.QueryerContext, postID string) (PostWithAuthor, error) {
	posts, err := selectPostsWithAuthor(ctx, db, `AND p.id = $1`, postID)
	if err != nil {
		return PostWithAuthor{}, err
	}
	if len(posts) == 0 {
		return PostWithAuthor{}, sql.ErrNoRows
	}
	return posts[0], nil
}

type viewerContext struct {
	User
	HasPostedToday   bool
	Friends          map[string]bool
	FriendsOfFriends map[string]bool
	MutualFriends    map[string]int
	Blocked          map[string]bool
}

func loadViewerContext(ctx context.Context, db sqlx.QueryerContext, viewerID string) (viewerContext, error) {
	return loadViewerContextWithFoF(ctx, db, viewerID, true, "")
}

func loadPostViewerContext(ctx context.Context, db sqlx.QueryerContext, viewerID string, post Post) (viewerContext, error) {
	// Other visibility modes (and the author's own posts) never consult FoF.
	return loadViewerContextWithFoF(ctx, db, viewerID, post.AuthorID != viewerID && post.Visibility == "friends_of_friends", post.AuthorID)
}

// An empty authorID loads all second-degree relationships for ranked/following
// feeds. A nonempty authorID only loads paths to that author. Direct friends,
// blocks, user status, and today's posting status always come from this query.
func loadViewerContextWithFoF(ctx context.Context, db sqlx.QueryerContext, viewerID string, includeFoF bool, authorID string) (viewerContext, error) {
	viewer := viewerContext{Friends: map[string]bool{}, FriendsOfFriends: map[string]bool{}, MutualFriends: map[string]int{}}
	friendshipsSQL := `'[]'::json`
	args := []any{viewerID}
	if includeFoF {
		where := `f.user_low_id = ANY(ARRAY(SELECT id FROM friends)) OR f.user_high_id = ANY(ARRAY(SELECT id FROM friends))`
		if authorID != "" {
			where = `(f.user_low_id = $2 AND f.user_high_id = ANY(ARRAY(SELECT id FROM friends)))
                OR (f.user_high_id = $2 AND f.user_low_id = ANY(ARRAY(SELECT id FROM friends)))`
			args = append(args, authorID)
		}
		friendshipsSQL = `(SELECT COALESCE(json_agg(json_build_array(f.user_low_id, f.user_high_id)), '[]')
     FROM friendships f WHERE ` + where + `)`
	}
	// europe/america から DB までは約100msかかるため、閲覧者の情報を1往復でまとめて取得する。
	var row struct {
		User
		HasPostedToday  bool   `db:"has_posted_today"`
		BlockedJSON     []byte `db:"blocked_json"`
		FriendsJSON     []byte `db:"friends_json"`
		FriendshipsJSON []byte `db:"friendships_json"`
	}
	if err := sqlx.GetContext(ctx, db, &row, `
  WITH viewer AS (
    SELECT `+strings.Join(userColumns, ", ")+` FROM users WHERE id = $1 AND status = 'active'
  ), friends AS (
    SELECT CASE WHEN user_low_id = $1 THEN user_high_id ELSE user_low_id END AS id
    FROM friendships WHERE user_low_id = $1 OR user_high_id = $1
  )
  SELECT viewer.*,
    EXISTS(
      SELECT 1 FROM posts p
      JOIN posting_windows posted ON posted.id = p.posting_window_id
      JOIN posting_windows current ON current.region = viewer.region AND current.ended_at IS NULL
                                  AND current.local_date = posted.local_date
      WHERE p.author_id = $1 AND p.deleted_at IS NULL
    ) AS has_posted_today,
    (SELECT COALESCE(json_agg(CASE WHEN blocker_id = $1 THEN blocked_id ELSE blocker_id END), '[]')
     FROM block_relations WHERE blocker_id = $1 OR blocked_id = $1) AS blocked_json,
    (SELECT COALESCE(json_agg(id), '[]') FROM friends) AS friends_json,
    `+friendshipsSQL+` AS friendships_json
  FROM viewer
 `, args...); err != nil {
		return viewer, err
	}
	viewer.User = row.User
	if err := viewer.User.setImageKey(); err != nil {
		return viewer, err
	}
	viewer.HasPostedToday = row.HasPostedToday
	var blocked, friends []string
	var friendships [][2]string
	if err := json.Unmarshal(row.BlockedJSON, &blocked); err != nil {
		return viewer, err
	}
	if err := json.Unmarshal(row.FriendsJSON, &friends); err != nil {
		return viewer, err
	}
	if err := json.Unmarshal(row.FriendshipsJSON, &friendships); err != nil {
		return viewer, err
	}
	viewer.Blocked = make(map[string]bool, len(blocked))
	for _, id := range blocked {
		viewer.Blocked[id] = true
	}
	for _, id := range friends {
		viewer.Friends[id] = true
	}
	for _, friendship := range friendships {
		low, high := friendship[0], friendship[1]
		if viewer.Friends[low] {
			viewer.FriendsOfFriends[high] = true
			if high != viewer.ID {
				viewer.MutualFriends[high]++
			}
		}
		if viewer.Friends[high] {
			viewer.FriendsOfFriends[low] = true
			if low != viewer.ID {
				viewer.MutualFriends[low]++
			}
		}
	}
	return viewer, nil
}

func (viewer viewerContext) canView(post Post) bool {
	if post.AuthorID == viewer.ID {
		return true
	}
	if post.WindowEndedAt != nil || !viewer.HasPostedToday || viewer.Blocked[post.AuthorID] {
		return false
	}
	switch post.Visibility {
	case "public":
		return true
	case "friends":
		return viewer.Friends[post.AuthorID]
	case "friends_of_friends":
		return viewer.Friends[post.AuthorID] || viewer.FriendsOfFriends[post.AuthorID]
	default:
		return false
	}
}

func canViewPost(ctx context.Context, db sqlx.QueryerContext, viewerID, postID string) (bool, error) {
	post, err := getPostWithAuthor(ctx, db, postID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	viewer, err := loadPostViewerContext(ctx, db, viewerID, post.Post)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return viewer.canView(post.Post), nil
}

func getFriendIDs(ctx context.Context, db sqlx.QueryerContext, userID string) ([]string, error) {
	var friendships []struct {
		UserLowID  string `db:"user_low_id"`
		UserHighID string `db:"user_high_id"`
	}
	if err := sqlx.SelectContext(ctx, db, &friendships, `SELECT user_low_id, user_high_id FROM friendships WHERE user_low_id = $1 OR user_high_id = $1`, userID); err != nil {
		return nil, err
	}
	var ids []string
	for _, friendship := range friendships {
		if friendship.UserLowID == userID {
			ids = append(ids, friendship.UserHighID)
		} else {
			ids = append(ids, friendship.UserLowID)
		}
	}
	return ids, nil
}

func getBlockedUserIDs(ctx context.Context, db sqlx.QueryerContext, userID string) (map[string]bool, error) {
	var ids []string
	if err := sqlx.SelectContext(ctx, db, &ids, `
  SELECT CASE WHEN blocker_id = $1 THEN blocked_id ELSE blocker_id END
  FROM block_relations WHERE blocker_id = $1 OR blocked_id = $1
 `, userID); err != nil {
		return nil, err
	}
	blocked := make(map[string]bool, len(ids))
	for _, id := range ids {
		blocked[id] = true
	}
	return blocked, nil
}
