package isureal

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v5"
)

type exportPostResponse struct {
	createPostResponse
	ImagePath string `json:"imagePath"`
}

type exportPost struct {
	Post
	ImageData  []byte  `db:"image_data"`
	StorageKey *string `db:"storage_key"`
}

// The locator supplies the partition key. Only an image absent from the locator
// uses the old all-partition lookup. Both branches see the same SQL snapshot.
const exportImageJoin = `
 LEFT JOIN image_locations loc ON loc.id=p.image_id
 JOIN LATERAL (
   SELECT i.* FROM image_objects i WHERE i.id=p.image_id AND i.created_at=loc.created_at
   UNION ALL
   SELECT i.* FROM image_objects i WHERE i.id=p.image_id AND loc.id IS NULL
 ) i ON true`

func selectExportPosts(ctx context.Context, db *sqlx.DB, userID string, storageColumn bool) ([]exportPost, error) {
	columns := postColumns + `, i.data AS image_data`
	if storageColumn {
		columns += `, i.storage_key`
	}
	base := `SELECT ` + columns + ` FROM posts p JOIN posting_windows w ON w.id=p.posting_window_id`
	conditions := ` WHERE p.author_id=$1 AND p.deleted_at IS NULL ORDER BY p.posted_at,p.id`
	var posts []exportPost
	err := sqlx.SelectContext(ctx, db, &posts, base+exportImageJoin+conditions, userID)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "42P01" {
		// Old deployments may start before reset.sql creates image_locations.
		err = sqlx.SelectContext(ctx, db, &posts, base+` JOIN image_objects i ON i.id=p.image_id`+conditions, userID)
	}
	return posts, err
}

func exportPostsHandler(h *HandlerContext) echo.HandlerFunc {
	return func(c *echo.Context) error {
		ctx := c.Request().Context()
		userID := GetSession(c).UserID
		author, err := getUser(ctx, h.DB, userID)
		if err != nil {
			return err
		}
		posts, err := selectExportPosts(ctx, h.DB, userID, h.StorageColumn || h.Files != nil)
		if err != nil {
			return err
		}

		var body bytes.Buffer
		writer := zip.NewWriter(&body)
		defer writer.Close()
		items := make([]exportPostResponse, 0, len(posts))
		for _, post := range posts {
			imagePath := fmt.Sprintf("images/%06d.jpg", len(items)+1)
			image, err := writer.CreateHeader(&zip.FileHeader{Name: imagePath, Method: zip.Store})
			if err != nil {
				return err
			}
			data, err := h.imageBytes(post.ImageData, post.StorageKey)
			if err != nil {
				return err
			}
			if _, err := image.Write(data); err != nil {
				return err
			}
			items = append(items, exportPostResponse{createPostResponse: newPostResponse(post.Post, author), ImagePath: imagePath})
		}
		manifest, err := writer.Create("posts.json")
		if err != nil {
			return err
		}
		if err := json.NewEncoder(manifest).Encode(items); err != nil {
			return err
		}
		if err := writer.Close(); err != nil {
			return err
		}
		c.Response().Header().Set(echo.HeaderContentDisposition, `attachment; filename="isureal-posts.zip"`)
		c.Response().Header().Set(echo.HeaderCacheControl, "no-store")
		return c.Blob(http.StatusOK, "application/zip", body.Bytes())
	}
}
