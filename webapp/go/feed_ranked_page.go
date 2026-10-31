package isureal

import (
	"context"
	"database/sql"
	"errors"
	"sort"

	"github.com/jmoiron/sqlx"
)

func getRankedFeed(ctx context.Context, db *sqlx.DB, viewerID string, input getFeedRequest) (getFeedResponse, error) {
	// Ranking and subsequent detail lookup must see the same users/posts/relations.
	// This read-only snapshot also prevents deleting or changing visibility between
	// candidate selection and materialization from producing an inconsistent page.
	tx, err := db.BeginTxx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return getFeedResponse{}, err
	}
	defer tx.Rollback()
	viewer, err := loadViewerContext(ctx, tx, viewerID)
	if errors.Is(err, sql.ErrNoRows) {
		return getFeedResponse{Items: []feedItemResponse{}}, nil
	}
	if err != nil {
		return getFeedResponse{}, err
	}
	if !viewer.HasPostedToday {
		return getFeedResponse{Items: []feedItemResponse{}}, nil
	}
	response, err := selectRankedPage(ctx, tx, viewer, input)
	if err != nil {
		return getFeedResponse{}, err
	}
	if err := tx.Commit(); err != nil {
		return getFeedResponse{}, err
	}
	return response, nil
}

func selectRankedPage(ctx context.Context, db *sqlx.Tx, viewer viewerContext, input getFeedRequest) (getFeedResponse, error) {
	response := getFeedResponse{Items: []feedItemResponse{}}
	// recommendationScores only needs ID and AuthorID; PostedAt provides the
	// existing tie-break/cursor order. No candidate limit before scoring.
	var candidates []PostWithAuthor
	if err := sqlx.SelectContext(ctx, db, &candidates, `
 SELECT p.id,p.author_id,p.posted_at FROM posts p
 JOIN posting_windows w ON w.id=p.posting_window_id
 JOIN users u ON u.id=p.author_id AND u.status='active'
 WHERE p.deleted_at IS NULL AND w.ended_at IS NULL AND p.author_id<>$1
 AND ($2::timestamptz IS NULL OR (p.posted_at,p.id)<($2,$3))
 `+rankedFeedConditions+` ORDER BY p.posted_at DESC,p.id DESC`, viewer.ID, nil, "", mapKeys(viewer.Blocked), mapKeys(viewer.Friends), mapKeys(viewer.FriendsOfFriends)); err != nil {
		return response, err
	}
	scores, err := rankedRecommendationScores(ctx, db, viewer, candidates)
	if err != nil {
		return response, err
	}
	sort.SliceStable(candidates, func(i, j int) bool { return scores[candidates[i].ID] > scores[candidates[j].ID] })
	ids := make([]string, 0, 51)
	for _, p := range candidates {
		if input.BeforeScore != nil && !rankedBefore(scores[p.ID], p.Post, *input.BeforeScore, *input.BeforePostedAt, input.BeforeID) {
			continue
		}
		ids = append(ids, p.ID)
		if len(ids) == 51 {
			break
		}
	}
	if len(ids) == 0 {
		return response, nil
	}
	posts, err := selectPostsWithAuthor(ctx, db, `AND p.id=ANY($1)`, ids)
	if err != nil {
		return response, err
	}
	byID := make(map[string]PostWithAuthor, len(posts))
	for _, p := range posts {
		byID[p.ID] = p
	}
	response.HasMore = len(ids) > 50
	for _, id := range ids[:min(50, len(ids))] {
		p, ok := byID[id]
		if !ok {
			return response, errors.New("ranked candidate missing from snapshot")
		}
		score := scores[id]
		response.Items = append(response.Items, feedItemResponse{createPostResponse: newPostResponse(p.Post, p.Author), Score: &score})
	}
	return response, nil
}
