package isureal

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image/jpeg"
	"net/http"

	"github.com/labstack/echo/v5"
)

const validationHeader = "X-Isureal-Image-Validation"
const cachedImageHeader = "X-Isureal-Cached-Image"

// An attestation states only that these exact bytes pass the original JPEG
// decoder and dimension limit. It never grants authentication or visibility.
func edgeMAC(key, domain, value string) string {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(domain))
	mac.Write([]byte{0})
	mac.Write([]byte(value))
	return hex.EncodeToString(mac.Sum(nil))
}
func validEdgeMAC(key, domain, value, proof string) bool {
	if len(key) < 32 {
		return false
	}
	expected := edgeMAC(key, domain, value)
	return hmac.Equal([]byte(expected), []byte(proof))
}
func imageValidationValue(data []byte, limit int) string {
	sum := sha256.Sum256(data)
	return fmt.Sprintf("%d:%x", limit, sum)
}

var errImageDimensions = errors.New("image dimensions exceed limit")

func decodeJPEG(data []byte, limit int) error {
	img, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		return err
	}
	if img.Bounds().Dx() > limit || img.Bounds().Dy() > limit {
		return errImageDimensions
	}
	return nil
}
func validateJPEG(c *echo.Context, h *HandlerContext, data []byte, limit int) error {
	key := ""
	if h.Config != nil {
		key = h.Config.EdgeKey
	}
	proof := c.Request().Header.Get(validationHeader)
	if len(key) >= 32 && proof != "" && validEdgeMAC(key, "jpeg", imageValidationValue(data, limit), proof) {
		return nil
	}
	return decodeJPEG(data, limit)
}
func hasCachedImageProof(r *http.Request, key string) bool {
	return validEdgeMAC(key, "cached-image", r.URL.RequestURI(), r.Header.Get(cachedImageHeader))
}
