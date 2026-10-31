package isureal

import (
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/labstack/echo/v5"
)

type getImageRequest struct {
	ImageID string `param:"imageId" json:"-"`
	Exp     string `query:"exp" json:"-"`
	Sig     string `query:"sig" json:"-"`
}

func getImageHandler(h *HandlerContext) echo.HandlerFunc {
	return func(c *echo.Context) error {
		c.Response().Header().Set("Cache-Control", "no-store")
		var input getImageRequest
		if err := c.Bind(&input); err != nil {
			return NewAPIError(http.StatusForbidden, "image_forbidden")
		}
		query, err := url.ParseQuery(c.Request().URL.RawQuery)
		if err != nil || len(query["exp"]) != 1 || len(query["sig"]) != 1 {
			return NewAPIError(http.StatusForbidden, "image_forbidden")
		}
		exp, err := strconv.ParseInt(input.Exp, 10, 64)
		if err != nil || strconv.FormatInt(exp, 10) != input.Exp || exp <= time.Now().Unix() {
			return NewAPIError(http.StatusForbidden, "image_forbidden")
		}
		sig, err := base64.RawURLEncoding.Strict().DecodeString(input.Sig)
		if err != nil || len(sig) != sha256.Size || base64.RawURLEncoding.EncodeToString(sig) != input.Sig {
			return NewAPIError(http.StatusForbidden, "image_forbidden")
		}

		ctx := c.Request().Context()
		_, generation := h.Images.GetGeneration(input.ImageID)
		metadataOnly := hasCachedImageProof(c.Request(), h.Config.EdgeKey)
		// 削除状態は毎回確認する。即時反映も5秒以内の許容範囲に含まれる。
		var object struct {
			OwnerUserID   string  `db:"owner_user_id"`
			SecretKeyHash string  `db:"secret_key_hash"`
			Visible       bool    `db:"visible"`
			Data          []byte  `db:"data"`
			StorageKey    *string `db:"storage_key"`
			MediaType     string  `db:"media_type"`
		}
		if cached := h.Images.Get(input.ImageID); cached != nil {
			// 本体はキャッシュにあるので、image_objects（36パーティション）を引かずに所有者と閲覧可否だけを確認する。
			object.OwnerUserID, object.MediaType, object.Data = cached.OwnerID, cached.MediaType, cached.Data
			err = h.DB.GetContext(ctx, &object, `
   SELECT u.secret_key_hash, COALESCE(
     u.profile_image_id = $1
     OR EXISTS (
       SELECT 1 FROM posts p
       WHERE p.image_id = $1 AND p.author_id = u.id AND p.deleted_at IS NULL
     )
     OR EXISTS (
       SELECT 1 FROM realmojis r
       JOIN posts p ON p.id = r.post_id AND p.deleted_at IS NULL
       JOIN users author ON author.id = p.author_id AND author.status = 'active'
       WHERE r.image_id = $1 AND r.reactor_id = u.id AND r.replaced_at IS NULL AND r.deleted_at IS NULL
     ), false) AS visible
   FROM users u
   WHERE u.id = $2 AND u.status = 'active'
  `, input.ImageID, cached.OwnerID)
		} else {
			// europe/america から DB までは約100msかかるため、署名鍵・閲覧可否・画像本体を1往復で取得する。
			// image_locations で created_at を引き、image_objects の36パーティションを実行時に1つへ絞る。
			query := imageMissQuery
			if h.StorageColumn || h.Files != nil {
				query = strings.Replace(query, "i.owner_user_id,", "i.storage_key, i.owner_user_id,", 1)
			}
			if metadataOnly {
				query = strings.Replace(query, "CASE WHEN v.visible THEN i.data END", "NULL::bytea", 1)
			}
			err = h.DB.GetContext(ctx, &object, query+` AND i.created_at = (SELECT created_at FROM image_locations WHERE id = $1)`, input.ImageID)
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "42P01" {
				// image_locations がまだ無い（reset.sql より先にアプリを再起動した）場合は、従来どおり全パーティションを探す。
				err = h.DB.GetContext(ctx, &object, query, input.ImageID)
			} else if errors.Is(err, sql.ErrNoRows) {
				// 対応表に無い（トリガー導入前の画像など）場合だけ、従来どおり全パーティションを探す。
				// 対応表にあって行が無い（所有者の退会など）ときは NOT EXISTS が偽になり、探索せずに終わる。
				err = h.DB.GetContext(ctx, &object, query+` AND NOT EXISTS (SELECT 1 FROM image_locations WHERE id = $1)`, input.ImageID)
			}
			if err == nil && object.Visible && !metadataOnly {
				object.Data, err = h.imageBytes(object.Data, object.StorageKey)
				if err != nil {
					return err
				}
				h.Images.PutGeneration(input.ImageID, &cachedImage{OwnerID: object.OwnerUserID, MediaType: object.MediaType, Data: object.Data}, generation)
			}
		}
		if errors.Is(err, sql.ErrNoRows) {
			return NewAPIError(http.StatusForbidden, "image_forbidden")
		}
		if err != nil {
			return err
		}
		key, err := imageSigningKey(object.SecretKeyHash)
		if err != nil {
			return err
		}
		if !hmac.Equal(sig, imageSignature(key, input.ImageID, exp)) {
			c.Set("abuse", "invalid_signature")
			return NewAPIError(http.StatusForbidden, "image_forbidden")
		}
		if !object.Visible {
			return NewAPIError(http.StatusForbidden, "image_forbidden")
		}
		if exp <= time.Now().Unix() {
			return NewAPIError(http.StatusForbidden, "image_forbidden")
		}
		if metadataOnly {
			return c.NoContent(http.StatusNoContent)
		}
		return c.Blob(http.StatusOK, object.MediaType, object.Data)
	}
}

const imageMissQuery = `
   SELECT i.owner_user_id, u.secret_key_hash, v.visible, CASE WHEN v.visible THEN i.data END AS data, i.media_type
   FROM image_objects i JOIN users u ON u.id = i.owner_user_id AND u.status = 'active'
   CROSS JOIN LATERAL (SELECT COALESCE(
     u.profile_image_id = i.id
     OR EXISTS (
       SELECT 1 FROM posts p
       WHERE p.image_id = i.id AND p.author_id = u.id AND p.deleted_at IS NULL
     )
     OR EXISTS (
       SELECT 1 FROM realmojis r
       JOIN posts p ON p.id = r.post_id AND p.deleted_at IS NULL
       JOIN users author ON author.id = p.author_id AND author.status = 'active'
       WHERE r.image_id = i.id AND r.reactor_id = u.id AND r.replaced_at IS NULL AND r.deleted_at IS NULL
     ), false) AS visible) v
   WHERE i.id = $1`
