package isureal

import (
	"bytes"
	"database/sql"
	"errors"
	"io"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
)

// This is an allocation hint cap, not a new upload limit. Unknown or larger
// bodies retain ReadAll's behavior. Reserve MinRead extra bytes so ReadFrom can
// observe EOF without growing again after an exactly Content-Length body.
func readProfileImageBody(body io.Reader, contentLength int64) ([]byte, error) {
	if contentLength <= 0 || contentLength > 32<<20 {
		return io.ReadAll(body)
	}
	var data bytes.Buffer
	data.Grow(int(contentLength) + bytes.MinRead)
	_, err := data.ReadFrom(body)
	return data.Bytes(), err
}

func putProfileImageHandler(h *HandlerContext) echo.HandlerFunc {
	return func(c *echo.Context) error {
		if c.Request().Header.Get("Content-Type") != "image/jpeg" {
			return NewAPIError(http.StatusUnsupportedMediaType, "unsupported_media_type")
		}
		data, err := readProfileImageBody(c.Request().Body, c.Request().ContentLength)
		if err != nil {
			return err
		}
		if err := validateJPEG(c, h, data, 512); err != nil {
			if !errors.Is(err, errImageDimensions) {
				c.Set("abuse", "invalid_image")
			}
			return NewAPIError(http.StatusBadRequest, "invalid_profile_image")
		}

		prepared, err := h.prepareImage(c.Request().Context(), data)
		if err != nil {
			return err
		}
		ctx := c.Request().Context()
		userID := GetSession(c).UserID
		tx, err := h.DB.BeginTxx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()
		var oldID *string
		if err := tx.GetContext(ctx, &oldID, `SELECT profile_image_id FROM users WHERE id=$1 FOR NO KEY UPDATE`, userID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return NewAPIError(http.StatusNotFound, "account_not_found")
			}
			return err
		}
		imageID := uuid.NewString()
		if err := h.insertImage(ctx, tx, imageID, userID, prepared, nil); err != nil {
			return err
		}
		h.Images.Put(imageID, &cachedImage{OwnerID: userID, MediaType: "image/jpeg", Data: data})
		if _, err := tx.ExecContext(ctx, `UPDATE users SET profile_image_id=$2,updated_at=clock_timestamp() WHERE id=$1`, userID, imageID); err != nil {
			return err
		}
		if oldID != nil {
			if _, err := tx.ExecContext(ctx, `DELETE FROM image_objects WHERE id=$1`, *oldID); err != nil {
				return err
			}
		}
		if err := tx.Commit(); err != nil {
			return err
		}
		return c.NoContent(http.StatusNoContent)
	}
}

func deleteProfileImageHandler(h *HandlerContext) echo.HandlerFunc {
	return func(c *echo.Context) error {
		ctx := c.Request().Context()
		userID := GetSession(c).UserID
		tx, err := h.DB.BeginTxx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()
		var imageID *string
		err = tx.GetContext(ctx, &imageID, `SELECT profile_image_id FROM users WHERE id=$1 FOR NO KEY UPDATE`, userID)
		if errors.Is(err, sql.ErrNoRows) {
			return NewAPIError(http.StatusNotFound, "account_not_found")
		}
		if err != nil {
			return err
		}
		if imageID == nil {
			return NewAPIError(http.StatusNotFound, "account_not_found")
		}
		if _, err := tx.ExecContext(ctx, `UPDATE users SET profile_image_id=NULL,updated_at=clock_timestamp() WHERE id=$1`, userID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM image_objects WHERE id=$1`, *imageID); err != nil {
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
		return c.NoContent(http.StatusNoContent)
	}
}
