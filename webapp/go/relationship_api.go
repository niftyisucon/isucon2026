package isureal

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
)

type deleteFriendshipRequest struct {
	UserID string `param:"userId" json:"-"`
}

func deleteFriendshipHandler(h *HandlerContext) echo.HandlerFunc {
	return func(c *echo.Context) error {
		var input deleteFriendshipRequest
		if err := c.Bind(&input); err != nil {
			return err
		}
		ctx := c.Request().Context()
		actor := GetSession(c).UserID
		tx, err := beginRelationshipTx(ctx, h.DB, actor, input.UserID)
		if err != nil {
			return err
		}
		defer tx.Rollback()
		result, err := tx.ExecContext(ctx, `DELETE FROM friendships WHERE user_low_id=$1 AND user_high_id=$2`, min(actor, input.UserID), max(actor, input.UserID))
		if err := requireRelationshipChange(result, err, http.StatusConflict, "relationship_conflict"); err != nil {
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
		return c.NoContent(http.StatusNoContent)
	}
}

type followRequest struct {
	UserID string `param:"userId" json:"-"`
}

func followHandler(h *HandlerContext) echo.HandlerFunc {
	return func(c *echo.Context) error {
		var input followRequest
		if err := c.Bind(&input); err != nil {
			return err
		}
		ctx := c.Request().Context()
		actor := GetSession(c).UserID
		tx, err := beginRelationshipTx(ctx, h.DB, actor, input.UserID)
		if err != nil {
			return err
		}
		defer tx.Rollback()
		result, err := tx.ExecContext(ctx, `
		 INSERT INTO follows (id, follower_id, followee_id)
		 SELECT $1::varchar(36),$2::varchar(36),$3::varchar(36)
		 WHERE EXISTS(SELECT 1 FROM users WHERE id=$3 AND account_type='certified_public')
		 AND NOT EXISTS(SELECT 1 FROM block_relations WHERE (blocker_id=$2 AND blocked_id=$3) OR (blocker_id=$3 AND blocked_id=$2))
		 ON CONFLICT (follower_id,followee_id) DO NOTHING`, uuid.NewString(), actor, input.UserID)
		if err := requireRelationshipChange(result, err, http.StatusConflict, "relationship_conflict"); err != nil {
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
		return c.NoContent(http.StatusNoContent)
	}
}

type unfollowRequest struct {
	UserID string `param:"userId" json:"-"`
}

func unfollowHandler(h *HandlerContext) echo.HandlerFunc {
	return func(c *echo.Context) error {
		var input unfollowRequest
		if err := c.Bind(&input); err != nil {
			return err
		}
		ctx := c.Request().Context()
		actor := GetSession(c).UserID
		tx, err := beginRelationshipTx(ctx, h.DB, actor, input.UserID)
		if err != nil {
			return err
		}
		defer tx.Rollback()
		result, err := tx.ExecContext(ctx, `DELETE FROM follows WHERE follower_id=$1 AND followee_id=$2`, actor, input.UserID)
		if err := requireRelationshipChange(result, err, http.StatusConflict, "relationship_conflict"); err != nil {
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
		return c.NoContent(http.StatusNoContent)
	}
}

type blockRequest struct {
	UserID string `param:"userId" json:"-"`
}

func blockHandler(h *HandlerContext) echo.HandlerFunc {
	return func(c *echo.Context) error {
		var input blockRequest
		if err := c.Bind(&input); err != nil {
			return err
		}
		ctx := c.Request().Context()
		actor := GetSession(c).UserID
		tx, err := beginRelationshipTx(ctx, h.DB, actor, input.UserID)
		if err != nil {
			return err
		}
		defer tx.Rollback()
		// All cleanup is conditional on inserting a new block. A duplicate block
		// must return conflict without changing requests or other relationships.
		var inserted int
		err = tx.GetContext(ctx, &inserted, `
		 WITH blocked AS (
		  INSERT INTO block_relations(id,blocker_id,blocked_id) VALUES($1,$2,$3)
		  ON CONFLICT (blocker_id,blocked_id) DO NOTHING RETURNING id
		 ), removed_friends AS (
		  DELETE FROM friendships WHERE user_low_id=$4 AND user_high_id=$5 AND EXISTS(SELECT 1 FROM blocked)
		 ), removed_follows AS (
		  DELETE FROM follows WHERE ((follower_id=$2 AND followee_id=$3) OR (follower_id=$3 AND followee_id=$2)) AND EXISTS(SELECT 1 FROM blocked)
		 ), cancelled_requests AS (
		  UPDATE friend_requests SET status='cancelled', updated_at=clock_timestamp()
		  WHERE status='pending' AND ((requester_id=$2 AND recipient_id=$3) OR (requester_id=$3 AND recipient_id=$2)) AND EXISTS(SELECT 1 FROM blocked)
		 ) SELECT count(*) FROM blocked`, uuid.NewString(), actor, input.UserID, min(actor, input.UserID), max(actor, input.UserID))
		if err != nil {
			return err
		}
		if inserted == 0 {
			return NewAPIError(http.StatusConflict, "relationship_conflict")
		}
		if err := tx.Commit(); err != nil {
			return err
		}
		return c.NoContent(http.StatusNoContent)
	}
}

type unblockRequest struct {
	UserID string `param:"userId" json:"-"`
}

func unblockHandler(h *HandlerContext) echo.HandlerFunc {
	return func(c *echo.Context) error {
		var input unblockRequest
		if err := c.Bind(&input); err != nil {
			return err
		}
		ctx := c.Request().Context()
		actor := GetSession(c).UserID
		tx, err := beginRelationshipTx(ctx, h.DB, actor, input.UserID)
		if err != nil {
			return err
		}
		defer tx.Rollback()
		result, err := tx.ExecContext(ctx, `DELETE FROM block_relations WHERE blocker_id=$1 AND blocked_id=$2`, actor, input.UserID)
		if err := requireRelationshipChange(result, err, http.StatusForbidden, "relationship_action_forbidden"); err != nil {
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
		return c.NoContent(http.StatusNoContent)
	}
}
