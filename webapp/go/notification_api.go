package isureal

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v5"
)

type getNotificationsResponse struct {
	ID              string    `json:"id" db:"id"`
	PostingWindowID string    `json:"postingWindowId" db:"posting_window_id"`
	Region          string    `json:"region" db:"region"`
	CreatedAt       time.Time `json:"createdAt" db:"created_at"`
}

func getNotificationsHandler(h *HandlerContext) echo.HandlerFunc {
	return func(c *echo.Context) error {
		ctx := c.Request().Context()
		session := GetSession(c)
		userID := session.UserID
		var connectedAt time.Time
		var epoch string
		if h.Notifications.epochSchema {
			if err := h.DB.QueryRowContext(ctx, `SELECT clock_timestamp(),generation::text FROM isureal_runtime_state WHERE id=true`).Scan(&connectedAt, &epoch); err != nil {
				return err
			}
		} else if err := h.DB.GetContext(ctx, &connectedAt, `SELECT clock_timestamp()`); err != nil {
			return err
		}
		response := c.Response()
		response.Header().Set("Content-Type", "text/event-stream")
		response.WriteHeader(http.StatusOK)
		controller := http.NewResponseController(response)
		if err := controller.Flush(); err != nil {
			return nil
		}
		sub := h.Notifications.subscribe(userID, session.Region, connectedAt, epoch)
		defer h.Notifications.unsubscribe(sub)
		select {
		case <-ctx.Done():
			return nil
		case notification := <-sub.result:
			if notification == nil {
				return nil
			}
			data, err := json.Marshal(notification)
			if err != nil {
				return err
			}
			if _, err := fmt.Fprintf(response, "data: %s\n\n", data); err != nil {
				return nil
			}
			_ = controller.Flush()
			return nil
		}
	}
}
