package isureal

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"time"
)

const imageURLLifetime = 24 * time.Hour
const imageSignatureIterations = 1000

// A/B rollback retains the signing cache while restoring the original expiry.
var imageURLExpiryBucket = configuredImageURLExpiryBucket(os.Getenv("ISUREAL_IMAGE_URL_EXPIRY_BUCKET_SECONDS"))

func configuredImageURLExpiryBucket(value string) time.Duration {
	switch value {
	case "", "60":
		return time.Minute
	case "1":
		return time.Second
	default:
		panic("ISUREAL_IMAGE_URL_EXPIRY_BUCKET_SECONDS must be 1 or 60")
	}
}

func imageURLExpiry(now time.Time, bucket time.Duration) int64 {
	// Round down: URLs remain valid for more than 23h59m and at most 24h.
	// Only issuance changes. GET still verifies the exact URL expiry and HMAC.
	return now.Add(imageURLLifetime).Truncate(bucket).Unix()
}

func imageSigningKey(secretKeyHash string) ([]byte, error) {
	key, err := hex.DecodeString(secretKeyHash)
	if err != nil || len(key) != sha256.Size {
		return nil, fmt.Errorf("invalid image signing key")
	}
	return key, nil
}

func imageSignature(key []byte, imageID string, exp int64) []byte {
	value := []byte(strconv.FormatInt(exp, 10) + ":" + imageID)
	mac := hmac.New(sha256.New, key)
	for range imageSignatureIterations {
		mac.Reset()
		mac.Write(value)
		value = mac.Sum(value[:0])
	}
	return value
}

func signedImageURL(user User, imageID string) string {
	exp := imageURLExpiry(time.Now(), imageURLExpiryBucket)
	if !imageURLCacheEnabled {
		return buildSignedImageURL(user.ImageKey, imageID, exp)
	}
	return signedImageURLs.get(user.ImageKey, imageID, exp)
}

func buildSignedImageURL(key []byte, imageID string, exp int64) string {
	query := url.Values{
		"exp": {strconv.FormatInt(exp, 10)},
		"sig": {base64.RawURLEncoding.EncodeToString(imageSignature(key, imageID, exp))},
	}
	return "/api/images/" + url.PathEscape(imageID) + "?" + query.Encode()
}
