package isureal

import (
	"database/sql"
	"errors"
	"net/http"
	"slices"
	"strings"

	"github.com/labstack/echo/v5"
	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
)

type searchUsersRequest struct {
	Q              string `query:"q"`
	BeforeUsername string `query:"beforeUsername"`
	BeforeID       string `query:"beforeId"`
}

type searchUsersResponse struct {
	Items   []relationshipUserResponse `json:"items"`
	HasMore bool                       `json:"hasMore"`
}

func normalizeSearchText(value string) string {
	return norm.NFC.String(cases.Fold().String(norm.NFC.String(value)))
}

func searchUsersHandler(h *HandlerContext) echo.HandlerFunc {
	return func(c *echo.Context) error {
		var input searchUsersRequest
		if err := c.Bind(&input); err != nil {
			return err
		}
		if !validUserText(input.Q, 1, 100) {
			return NewAPIError(http.StatusBadRequest, "invalid_relationship")
		}
		if (input.BeforeUsername == "") != (input.BeforeID == "") {
			return NewAPIError(http.StatusBadRequest, "invalid_pagination")
		}
		ctx := c.Request().Context()
		viewer := GetSession(c).UserID
		// Keep Unicode matching in Go: SQL lower/ILIKE does not implement
		// normalizeSearchText's NFC + Unicode case folding. Transfer only the
		// search fields, then load response/signing data for at most 51 matches.
		var candidates []struct {
			ID          string `db:"id"`
			Username    string `db:"username"`
			DisplayName string `db:"display_name"`
		}
		if err := h.DB.SelectContext(ctx, &candidates, `SELECT id, username, display_name FROM users
			WHERE status='active' AND id<>$1 AND ($2='' OR (username,id)>($2,$3))
			AND id NOT IN (SELECT CASE WHEN blocker_id=$1 THEN blocked_id ELSE blocker_id END
				FROM block_relations WHERE blocker_id=$1 OR blocked_id=$1)
			ORDER BY username, id`, viewer, input.BeforeUsername, input.BeforeID); err != nil {
			return err
		}
		query := normalizeSearchText(input.Q)
		ids := make([]string, 0, 51)
		for _, user := range candidates {
			if strings.Contains(normalizeSearchText(user.Username), query) || strings.Contains(normalizeSearchText(user.DisplayName), query) {
				ids = append(ids, user.ID)
				if len(ids) == 51 {
					break
				}
			}
		}
		var users []User
		if len(ids) > 0 {
			if err := h.DB.SelectContext(ctx, &users, `SELECT `+strings.Join(userColumns, ", ")+` FROM users WHERE id=ANY($1) AND status='active'
				AND id NOT IN (SELECT CASE WHEN blocker_id=$2 THEN blocked_id ELSE blocker_id END
					FROM block_relations WHERE blocker_id=$2 OR blocked_id=$2)
				ORDER BY username, id`, ids, viewer); err != nil {
				return err
			}
		}
		items := make([]relationshipUserResponse, 0, len(users))
		for _, user := range users {
			if err := user.setImageKey(); err != nil {
				return err
			}
			if user.AccountType != "certified_public" {
				user.Bio = ""
			}
			items = append(items, newRelationshipUserResponse(user))
			if len(items) == 51 {
				break
			}
		}
		hasMore := len(items) > 50
		if hasMore {
			items = items[:50]
		}
		return c.JSON(http.StatusOK, searchUsersResponse{Items: items, HasMore: hasMore})
	}
}

type getUserRequest struct {
	UserID string `param:"userId"`
}

type getUserResponse relationshipUserResponse

func getUserHandler(h *HandlerContext) echo.HandlerFunc {
	return func(c *echo.Context) error {
		var input getUserRequest
		if err := c.Bind(&input); err != nil {
			return err
		}
		ctx := c.Request().Context()
		viewer := GetSession(c).UserID
		user, err := getUser(ctx, h.DB, input.UserID)
		if errors.Is(err, sql.ErrNoRows) {
			return NewAPIError(http.StatusNotFound, "relationship_not_found")
		}
		if err != nil {
			return err
		}
		blocked, err := isBlocked(ctx, h.DB, viewer, user.ID)
		if err != nil {
			return err
		}
		if blocked {
			return NewAPIError(http.StatusNotFound, "relationship_not_found")
		}
		friends, err := getFriendIDs(ctx, h.DB, viewer)
		if err != nil {
			return err
		}
		if user.AccountType != "certified_public" && !slices.Contains(friends, user.ID) {
			user.Bio = ""
		}
		return c.JSON(http.StatusOK, getUserResponse(newRelationshipUserResponse(user)))
	}
}

func newRelationshipUserResponse(user User) relationshipUserResponse {
	return relationshipUserResponse{ID: user.ID, Username: user.Username, DisplayName: user.DisplayName, ProfileImageURL: newGetMeResponse(user).ProfileImageURL, Bio: user.Bio, AccountType: user.AccountType}
}
