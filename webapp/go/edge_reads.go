package isureal

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/labstack/echo/v5"
)

// Only register on edges. A failed probe has no response, session, ban or
// authorization-cache side effects; the existing proxy handles it in full.
func registerEdgeReads(e *echo.Echo, h *HandlerContext, proxy echo.HandlerFunc) {
	if os.Getenv("ISUREAL_EDGE_IMAGE_VERIFY") == "true" {
		e.GET("/api/images/:imageId", edgeImageHandler(h, proxy))
	}
	if os.Getenv("ISUREAL_EDGE_READS") == "true" {
		e.GET("/api/posting-window/current", edgeCurrentReadHandler(h, proxy, true))
		e.GET("/api/posts/me/current", edgeCurrentReadHandler(h, proxy, false))
	}
}

func edgeImageHandler(h *HandlerContext, proxy echo.HandlerFunc) echo.HandlerFunc {
	// Match the core's metadata-only miss query, including partition pruning.
	query := strings.Replace(imageMissQuery, "CASE WHEN v.visible THEN i.data END", "NULL::bytea", 1) + `
 AND i.created_at = (SELECT created_at FROM image_locations WHERE id = $1)
 AND NOT EXISTS (SELECT 1 FROM ip_bans WHERE ip = $2 AND banned_until > clock_timestamp())`
	return func(c *echo.Context) error {
		id := edgeImageID(c.Request())
		cached := h.Images.Get(id)
		if id == "" || cached == nil {
			return proxy(c)
		}
		var input getImageRequest
		if c.Bind(&input) != nil || input.ImageID != id {
			return proxy(c)
		}
		values, err := url.ParseQuery(c.Request().URL.RawQuery)
		if err != nil || len(values["exp"]) != 1 || len(values["sig"]) != 1 {
			return proxy(c)
		}
		exp, err := strconv.ParseInt(input.Exp, 10, 64)
		if err != nil || strconv.FormatInt(exp, 10) != input.Exp || exp <= time.Now().Unix() {
			return proxy(c)
		}
		sig, err := base64.RawURLEncoding.Strict().DecodeString(input.Sig)
		if err != nil || len(sig) != sha256.Size || base64.RawURLEncoding.EncodeToString(sig) != input.Sig {
			return proxy(c)
		}
		var object struct {
			OwnerUserID   string `db:"owner_user_id"`
			SecretKeyHash string `db:"secret_key_hash"`
			Visible       bool   `db:"visible"`
			Data          []byte `db:"data"`
			MediaType     string `db:"media_type"`
		}
		// Avoid pgx's first-use prepare/describe round trip on edge connections.
		if h.DB.GetContext(c.Request().Context(), &object, query, pgx.QueryExecModeExec, id, c.RealIP()) != nil {
			return proxy(c)
		}
		key, err := imageSigningKey(object.SecretKeyHash)
		// A mismatch must reach core so its abuse logging and ban behavior run.
		if err != nil || !hmac.Equal(sig, imageSignature(key, id, exp)) || !object.Visible ||
			(cached.OwnerID != "" && cached.OwnerID != object.OwnerUserID) || cached.MediaType != object.MediaType || exp <= time.Now().Unix() {
			return proxy(c)
		}
		c.Response().Header().Set("Cache-Control", "no-store")
		return c.Blob(http.StatusOK, object.MediaType, cached.Data)
	}
}

// The live session, current region and IP ban are checked in the same statement
// as the response data. No transaction or session/visibility/ban cache is used.
const edgeReadConditions = `
 s.token_hash = $1 AND s.active = true AND u.status = 'active' AND u.region = $2
 AND NOT EXISTS (SELECT 1 FROM ip_bans WHERE ip = $3 AND banned_until > clock_timestamp())`

func edgeCurrentReadHandler(h *HandlerContext, proxy echo.HandlerFunc, window bool) echo.HandlerFunc {
	return func(c *echo.Context) error {
		auth := c.Request().Header.Get(echo.HeaderAuthorization)
		if !strings.HasPrefix(auth, "Bearer ") {
			return proxy(c)
		}
		hash := sha256.Sum256([]byte(strings.TrimPrefix(auth, "Bearer ")))
		args := []any{pgx.QueryExecModeExec, hex.EncodeToString(hash[:]), h.Config.Region, c.RealIP()}
		var response any
		var userID string
		if window {
			var state struct {
				PostingWindow
				UserID    string `db:"user_id"`
				HasWindow bool   `db:"has_window"`
				Active    int    `db:"active"`
				Deleted   int    `db:"deleted"`
				CanPost   bool   `db:"can_post"`
			}
			err := h.DB.GetContext(c.Request().Context(), &state, `
 SELECT u.id AS user_id, w.id IS NOT NULL AS has_window, COALESCE(w.id, '') AS id, COALESCE(w.region, '') AS region,
        COALESCE(w.local_date, '1970-01-01'::date) AS local_date,
        COALESCE(w.starts_at, 'epoch'::timestamptz) AS starts_at,
        COALESCE(w.on_time_deadline, 'epoch'::timestamptz) AS on_time_deadline, w.ended_at,
        count(p.id) FILTER (WHERE p.deleted_at IS NULL) AS active,
        count(p.id) FILTER (WHERE p.deleted_at IS NOT NULL) AS deleted,
        COALESCE(count(p.id) FILTER (WHERE p.deleted_at IS NULL) = 0
          AND count(p.id) FILTER (WHERE p.deleted_at IS NOT NULL) < 2
          AND clock_timestamp() >= w.starts_at, false) AS can_post
 FROM login_sessions s JOIN users u ON u.id = s.user_id
 LEFT JOIN posting_windows w ON w.region = u.region AND w.ended_at IS NULL
 LEFT JOIN posting_windows posted_window ON posted_window.local_date = w.local_date
 LEFT JOIN posts p ON p.author_id = u.id AND p.posting_window_id = posted_window.id
 WHERE `+edgeReadConditions+`
 GROUP BY u.id, w.id`, args...)
			if err != nil {
				return proxy(c)
			}
			out := getCurrentPostingWindowResponse{}
			if state.HasWindow {
				out = getCurrentPostingWindowResponse{
					Window: &currentPostingWindowResponse{
						ID: state.ID, Region: state.Region, LocalDate: state.LocalDate.Format(time.DateOnly),
						StartsAt: state.StartsAt, OnTimeDeadline: state.OnTimeDeadline,
					},
					HasPosted: state.Active > 0, CanPost: state.CanPost, DeletionCount: state.Deleted,
				}
			}
			response, userID = out, state.UserID
		} else {
			var post PostWithAuthor
			err := h.DB.GetContext(c.Request().Context(), &post, `
 SELECT `+postColumns+`, `+userColumnsAs("u", "author")+`
 FROM login_sessions s JOIN users u ON u.id = s.user_id
 JOIN posts p ON p.author_id = u.id AND p.deleted_at IS NULL
 JOIN posting_windows w ON w.id = p.posting_window_id
 JOIN posting_windows current ON current.region = u.region AND current.ended_at IS NULL
                            AND current.local_date = w.local_date
 WHERE `+edgeReadConditions, args...)
			if err != nil || post.Author.setImageKey() != nil {
				return proxy(c)
			}
			response = getMyCurrentPostResponse(newPostResponse(post.Post, post.Author))
			userID = post.Author.ID
		}
		// Marshal before touching the response, so unsupported timestamps also
		// fall back. Echo's default JSON encoder uses the same bytes plus newline.
		data, err := json.Marshal(response)
		if err != nil {
			return proxy(c)
		}
		c.Set("session", &Session{UserID: userID, Region: h.Config.Region})
		return c.JSONBlob(http.StatusOK, append(data, '\n'))
	}
}
