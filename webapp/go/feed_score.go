package isureal

import (
	"context"

	"github.com/jmoiron/sqlx"
)

// Local sparse-data measurements still favor the simple queries at 64 posts.
// Keep a margin before switching; the production crossover is workload-dependent.
const scoreAggregateThreshold = 256

func rankedRecommendationScores(ctx context.Context, tx *sqlx.Tx, viewer viewerContext, posts []PostWithAuthor) (map[string]int64, error) {
	if len(posts) < scoreAggregateThreshold {
		return recommendationScores(ctx, tx, viewer, posts)
	}
	// Candidate/friend array sizes vary by viewer. A cached generic plan can
	// underestimate them badly. Limit custom planning to this transaction;
	// SET + one aggregate query keeps the old two score-query round trips.
	if _, err := tx.ExecContext(ctx, `SET LOCAL plan_cache_mode=force_custom_plan`); err != nil {
		return nil, err
	}
	return aggregatedRecommendationScores(ctx, tx, viewer, posts)
}

// Score every candidate in one statement, returning one scalar per post rather
// than transferring all friendships and reactions for Go-side aggregation.
// The caller's repeatable-read snapshot covers viewer context and pagination.
const recommendationScoresSQL = `
 WITH candidates AS MATERIALIZED (
   SELECT * FROM unnest($1::text[], $2::text[]) AS p(post_id, author_id)
 ), authors AS (
   SELECT DISTINCT author_id FROM candidates
 ), friends AS (
   SELECT unnest($3::text[]) AS user_id
 ), mutual AS (
   SELECT * FROM unnest($4::text[], $5::bigint[]) AS m(user_id, weight)
 ), weighted_edges AS MATERIALIZED (
   SELECT f.user_low_id AS author_id, m.weight
   FROM friendships f JOIN authors a ON a.author_id=f.user_low_id
   JOIN mutual m ON m.user_id=f.user_high_id
   UNION ALL
   SELECT f.user_high_id AS author_id, m.weight
   FROM friendships f JOIN authors a ON a.author_id=f.user_high_id
   JOIN mutual m ON m.user_id=f.user_low_id
 ), connections AS MATERIALIZED (
   SELECT author_id, LEAST(sum(weight),30) AS score FROM weighted_edges GROUP BY author_id
 ), engagements AS MATERIALIZED (
   SELECT r.post_id, r.reactor_id AS user_id
   FROM realmojis r JOIN candidates p ON p.post_id=r.post_id
   JOIN friends f ON f.user_id=r.reactor_id
   JOIN users u ON u.id=r.reactor_id AND u.status='active'
   WHERE r.replaced_at IS NULL AND r.deleted_at IS NULL
   UNION
   SELECT c.post_id, c.author_id AS user_id
   FROM comments c JOIN candidates p ON p.post_id=c.post_id
   JOIN friends f ON f.user_id=c.author_id
   JOIN users u ON u.id=c.author_id AND u.status='active'
   WHERE c.deleted_at IS NULL
 ), engagement_scores AS MATERIALIZED (
   SELECT post_id, count(*) AS friend_count FROM engagements GROUP BY post_id
 )
 SELECT p.post_id, (10*COALESCE(e.friend_count,0) + 3*COALESCE(m.weight,0) + COALESCE(c.score,0))::bigint AS score
 FROM candidates p
 LEFT JOIN mutual m ON m.user_id=p.author_id
 LEFT JOIN connections c ON c.author_id=p.author_id
 LEFT JOIN engagement_scores e ON e.post_id=p.post_id
`

func aggregatedRecommendationScores(ctx context.Context, db sqlx.QueryerContext, viewer viewerContext, posts []PostWithAuthor) (map[string]int64, error) {
	scores := make(map[string]int64, len(posts))
	if len(posts) == 0 {
		return scores, nil
	}
	postIDs, authorIDs := make([]string, 0, len(posts)), make([]string, 0, len(posts))
	for _, post := range posts {
		postIDs = append(postIDs, post.ID)
		authorIDs = append(authorIDs, post.AuthorID)
	}
	friends := make([]string, 0, len(viewer.Friends))
	for id, friend := range viewer.Friends {
		if friend {
			friends = append(friends, id)
		}
	}
	mutualIDs, weights := make([]string, 0, len(viewer.MutualFriends)), make([]int64, 0, len(viewer.MutualFriends))
	for id, weight := range viewer.MutualFriends {
		mutualIDs = append(mutualIDs, id)
		weights = append(weights, int64(weight))
	}
	var rows []struct {
		PostID string `db:"post_id"`
		Score  int64  `db:"score"`
	}
	if err := sqlx.SelectContext(ctx, db, &rows, recommendationScoresSQL, postIDs, authorIDs, friends, mutualIDs, weights); err != nil {
		return nil, err
	}
	for _, row := range rows {
		scores[row.PostID] = row.Score
	}
	return scores, nil
}
