package isureal

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"
)

func TestProfileImageErrorConsumesRequestBody(t *testing.T) {
	body := bytes.NewReader(bytes.Repeat([]byte("x"), 1<<20))
	req := httptest.NewRequest(http.MethodPut, "/api/me/profile-image", body)
	rec := httptest.NewRecorder()
	c := echo.New().NewContext(req, rec)
	APIErrorHandler(c, NewAPIError(http.StatusUnauthorized, "invalid_session"))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", rec.Code)
	}
	remaining, err := io.ReadAll(req.Body)
	if err != nil || len(remaining) != 0 {
		t.Fatalf("remaining body = %d bytes, error = %v", len(remaining), err)
	}
}
