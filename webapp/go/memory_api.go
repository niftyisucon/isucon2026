package isureal

import (
	"context"
	"net/http"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v5"
)

type getMemoriesRequest struct {
	Month          string     `query:"month"`
	BeforePostedAt *time.Time `query:"beforePostedAt"`
	BeforeID       string     `query:"beforeId"`
}

type memoryPostResponse struct {
	createPostResponse
	RealMojis []replaceRealMojiResponse `json:"realmojis"`
	Comments  []createCommentResponse   `json:"comments"`
}

type getMemoriesResponse struct {
	Items   []memoryPostResponse `json:"items"`
	HasMore bool                 `json:"hasMore"`
}

type getMemoriesByDateRequest struct {
	Date           string     `param:"date"`
	BeforePostedAt *time.Time `query:"beforePostedAt"`
	BeforeID       string     `query:"beforeId"`
}

type getMemoriesByDateResponse getMemoriesResponse

func getMemoriesHandler(h *HandlerContext) echo.HandlerFunc {
	return func(c *echo.Context) error {
		var input getMemoriesRequest
		if err := c.Bind(&input); err != nil {
			return err
		}
		start, err := time.Parse("2006-01", input.Month)
		if err != nil {
			return NewAPIError(http.StatusBadRequest, "invalid_memory_period")
		}
		return respondMemories(c, h, start, start.AddDate(0, 1, 0), input.BeforePostedAt, input.BeforeID)
	}
}

func getMemoriesByDateHandler(h *HandlerContext) echo.HandlerFunc {
	return func(c *echo.Context) error {
		var input getMemoriesByDateRequest
		if err := c.Bind(&input); err != nil {
			return err
		}
		start, err := time.Parse(time.DateOnly, input.Date)
		if err != nil {
			return NewAPIError(http.StatusBadRequest, "invalid_memory_period")
		}
		return respondMemories(c, h, start, start.AddDate(0, 0, 1), input.BeforePostedAt, input.BeforeID)
	}
}

func respondMemories(c *echo.Context, h *HandlerContext, start, end time.Time, beforeAt *time.Time, beforeID string) error {
	if h.Config != nil && edgeMemoryRenderingEnabled() && wantsEdgeMemory(c.Request(), h.Config.EdgeKey) {
		data, err := loadMemoryData(c.Request().Context(), h.DB, GetSession(c).UserID, start, end, beforeAt, beforeID)
		if err != nil {
			return err
		}
		body, err := sealMemoryData(data, h.Config.EdgeKey, memoryRequestBinding(c.Request()))
		if err != nil {
			return err
		}
		if body != nil {
			c.Response().Header().Set("Cache-Control", "no-store")
			return c.Blob(http.StatusOK, edgeMemoryContentType, body)
		}
		// Large models retain the ordinary response without imposing a new API limit.
		return c.JSON(http.StatusOK, renderMemoryData(data))
	}
	response, err := listMemories(c.Request().Context(), h.DB, GetSession(c).UserID, start, end, beforeAt, beforeID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, response)
}

func listMemories(ctx context.Context, db sqlx.QueryerContext, userID string, start, end time.Time, beforeAt *time.Time, beforeID string) (getMemoriesResponse, error) {
	var response getMemoriesResponse
	if (beforeAt == nil) != (beforeID == "") {
		return response, NewAPIError(http.StatusBadRequest, "invalid_pagination")
	}
	posts, err := selectPostsWithAuthor(ctx, db, `
  AND p.author_id = $1 AND w.local_date >= $2 AND w.local_date < $3
  AND ($4::timestamptz IS NULL OR (p.posted_at, p.id) < ($4, $5))
  ORDER BY p.posted_at DESC, p.id DESC LIMIT 51
 `, userID, start, end, beforeAt, beforeID)
	if err != nil {
		return response, err
	}
	items := make([]memoryPostResponse, 0, len(posts))
	for _, post := range posts {
		items = append(items, memoryPostResponse{
			createPostResponse: newPostResponse(post.Post, post.Author),
			RealMojis:          make([]replaceRealMojiResponse, 0),
			Comments:           make([]createCommentResponse, 0),
		})
	}
	response.HasMore = len(items) > 50
	if response.HasMore {
		items = items[:50]
	}
	if err := loadMemoryInteractions(ctx, db, items); err != nil {
		return response, err
	}
	response.Items = items
	return response, nil
}

func loadMemoryInteractions(ctx context.Context, db sqlx.QueryerContext, items []memoryPostResponse) error {
	if len(items) == 0 {
		return nil
	}
	postIDs := make([]string, len(items))
	itemByPostID := make(map[string]*memoryPostResponse, len(items))
	for i := range items {
		postIDs[i] = items[i].ID
		itemByPostID[items[i].ID] = &items[i]
	}

	// Bound each post after excluding inactive reactors and non-current rows.
	reactions, err := selectCurrentRealMojis(ctx, db, memoryReactionConditions, postIDs)
	if err != nil {
		return err
	}
	for _, reaction := range reactions {
		item := itemByPostID[reaction.PostID]
		if len(item.RealMojis) < 50 {
			item.RealMojis = append(item.RealMojis, newRealMojiResponse(reaction.RealMoji, reaction.Reactor))
		}
	}

	comments, err := selectCommentsWithAuthor(ctx, db, memoryCommentConditions, postIDs)
	if err != nil {
		return err
	}
	for _, comment := range comments {
		item := itemByPostID[comment.PostID]
		if len(item.Comments) < 50 {
			item.Comments = append(item.Comments, newCommentResponse(comment.Comment, comment.Author))
		}
	}
	return nil
}

// Keep each kind in one round trip; the lateral subquery limits each post,
// not the complete page. The outer helpers retain response and signing logic.
const memoryReactionConditions = `AND r.id IN (
 SELECT picked.id FROM unnest($1::text[]) AS selected(post_id)
 CROSS JOIN LATERAL (
  SELECT candidate.id FROM realmojis candidate
  JOIN users actor ON actor.id=candidate.reactor_id AND actor.status='active'
  WHERE candidate.post_id=selected.post_id AND candidate.replaced_at IS NULL AND candidate.deleted_at IS NULL
  ORDER BY candidate.created_at DESC, candidate.id DESC LIMIT 50
 ) picked
) ORDER BY r.created_at DESC, r.id DESC`

const memoryCommentConditions = `AND c.id IN (
 SELECT picked.id FROM unnest($1::text[]) AS selected(post_id)
 CROSS JOIN LATERAL (
  SELECT candidate.id FROM comments candidate
  JOIN users actor ON actor.id=candidate.author_id AND actor.status='active'
  WHERE candidate.post_id=selected.post_id AND candidate.deleted_at IS NULL
  ORDER BY candidate.created_at, candidate.id LIMIT 50
 ) picked
) ORDER BY c.created_at, c.id`
