package isureal

import (
	"database/sql"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
)

type createFriendRequestRequest struct {
	RecipientID string `json:"recipientId"`
}

type createFriendRequestResponse struct {
	RelationID string    `json:"relationId"`
	Status     string    `json:"status"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

func createFriendRequestHandler(h *HandlerContext) echo.HandlerFunc {
	return func(c *echo.Context) error {
		var input createFriendRequestRequest
		if err := c.Bind(&input); err != nil {
			return err
		}
		ctx := c.Request().Context()
		actor := GetSession(c).UserID
		tx, err := beginRelationshipTx(ctx, h.DB, actor, input.RecipientID)
		if err != nil {
			return err
		}
		defer tx.Rollback()
		var state struct {
			Blocked          bool           `db:"blocked"`
			Friends          bool           `db:"friends"`
			PendingID        sql.NullString `db:"pending_id"`
			PendingRequester sql.NullString `db:"pending_requester"`
		}
		if err := tx.GetContext(ctx, &state, `SELECT
		 EXISTS(SELECT 1 FROM block_relations WHERE (blocker_id=$1 AND blocked_id=$2) OR (blocker_id=$2 AND blocked_id=$1)) AS blocked,
		 EXISTS(SELECT 1 FROM friendships WHERE user_low_id=$3 AND user_high_id=$4) AS friends,
		 pending.id AS pending_id, pending.requester_id AS pending_requester
		 FROM (SELECT 1) anchor LEFT JOIN LATERAL (
		   SELECT id,requester_id FROM friend_requests WHERE status='pending' AND
		   ((requester_id=$1 AND recipient_id=$2) OR (requester_id=$2 AND recipient_id=$1))
		 ) pending ON true`, actor, input.RecipientID, min(actor, input.RecipientID), max(actor, input.RecipientID)); err != nil {
			return err
		}
		if state.Blocked || state.Friends {
			return NewAPIError(http.StatusConflict, "relationship_conflict")
		}
		var request FriendRequest
		if state.PendingID.Valid {
			if state.PendingRequester.String == actor {
				return NewAPIError(http.StatusConflict, "relationship_conflict")
			}
			request, err = acceptFriendRequestLocked(ctx, tx, state.PendingID.String, actor, input.RecipientID)
			if err != nil {
				return err
			}
		} else {
			requestID := uuid.NewString()
			if err := tx.GetContext(ctx, &request, `INSERT INTO friend_requests (id, requester_id, recipient_id, status, created_at, updated_at) VALUES ($1,$2,$3,'pending',clock_timestamp(),clock_timestamp()) RETURNING id,requester_id,recipient_id,status,created_at,updated_at`, requestID, actor, input.RecipientID); err != nil {
				return err
			}
		}
		if err := tx.Commit(); err != nil {
			return err
		}
		return c.JSON(http.StatusCreated, createFriendRequestResponse{RelationID: request.ID, Status: request.Status, UpdatedAt: request.UpdatedAt})
	}
}

type transitionFriendRequestRequest struct {
	RequestID string `param:"requestId" json:"-"`
	Action    string `param:"action" json:"-"`
}

type transitionFriendRequestResponse createFriendRequestResponse

func transitionFriendRequestHandler(h *HandlerContext) echo.HandlerFunc {
	return func(c *echo.Context) error {
		var input transitionFriendRequestRequest
		if err := c.Bind(&input); err != nil {
			return err
		}
		var status string
		switch input.Action {
		case "accept":
			status = "accepted"
		case "reject":
			status = "rejected"
		case "cancel":
			status = "cancelled"
		default:
			return NewAPIError(http.StatusForbidden, "relationship_action_forbidden")
		}
		ctx := c.Request().Context()
		actor := GetSession(c).UserID
		request, err := getFriendRequest(ctx, h.DB, input.RequestID)
		if errors.Is(err, sql.ErrNoRows) {
			return NewAPIError(http.StatusNotFound, "relationship_not_found")
		}
		if err != nil {
			return err
		}
		if (status == "cancelled" && actor != request.RequesterID) || (status != "cancelled" && actor != request.RecipientID) {
			return NewAPIError(http.StatusForbidden, "relationship_action_forbidden")
		}
		tx, err := beginRelationshipTx(ctx, h.DB, request.RequesterID, request.RecipientID)
		if err != nil {
			return err
		}
		defer tx.Rollback()
		var state struct {
			FriendRequest
			Blocked bool `db:"blocked"`
		}
		// This is a new statement after both user locks, so a lock wait cannot
		// leave us evaluating relationship state from a pre-wait snapshot.
		if err := tx.GetContext(ctx, &state, `
		 SELECT fr.id,fr.requester_id,fr.recipient_id,fr.status,fr.created_at,fr.updated_at,
		 CASE WHEN $2='accepted' THEN EXISTS (
		   SELECT 1 FROM block_relations WHERE
		   (blocker_id=fr.requester_id AND blocked_id=fr.recipient_id) OR
		   (blocker_id=fr.recipient_id AND blocked_id=fr.requester_id)
		 ) ELSE false END AS blocked FROM friend_requests fr WHERE fr.id=$1`, input.RequestID, status); err != nil {
			return err
		}
		request = state.FriendRequest
		if request.Status != "pending" {
			return NewAPIError(http.StatusForbidden, "relationship_action_forbidden")
		}
		if status == "accepted" {
			if state.Blocked {
				return NewAPIError(http.StatusConflict, "relationship_conflict")
			}
			request, err = acceptFriendRequestLocked(ctx, tx, request.ID, request.RequesterID, request.RecipientID)
			if err != nil {
				return err
			}
		} else if err := tx.GetContext(ctx, &request, `UPDATE friend_requests SET status=$2, updated_at=clock_timestamp() WHERE id=$1 RETURNING id,requester_id,recipient_id,status,created_at,updated_at`, request.ID, status); err != nil {
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
		return c.JSON(http.StatusOK, transitionFriendRequestResponse{RelationID: request.ID, Status: request.Status, UpdatedAt: request.UpdatedAt})
	}
}
