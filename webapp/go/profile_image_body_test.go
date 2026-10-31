package isureal

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"
)

type profileBodyReader struct {
	data []byte
	step int
	err  error
}

func (r *profileBodyReader) Read(p []byte) (int, error) {
	n := min(len(p), len(r.data), r.step)
	copy(p, r.data[:n])
	r.data = r.data[n:]
	if len(r.data) == 0 {
		return n, r.err
	}
	return n, nil
}

func TestProfileImageBodyMatchesReadAll(t *testing.T) {
	input := bytes.Repeat([]byte{0xff, 0xd8, 0x00, 0x7f}, 1024)
	failed := errors.New("body read failed")
	for _, size := range []int{0, 1, 511, 512, 513, len(input)} {
		for _, hint := range []int64{-1, 0, 1, int64(size), int64(size + 1024), 32 << 20, (32 << 20) + 1, math.MaxInt64} {
			for _, end := range []error{io.EOF, io.ErrUnexpectedEOF, failed} {
				t.Run(fmt.Sprintf("size=%d/hint=%d/end=%v", size, hint, end), func(t *testing.T) {
					want, wantErr := io.ReadAll(&profileBodyReader{input[:size], 17, end})
					got, gotErr := readProfileImageBody(&profileBodyReader{input[:size], 17, end}, hint)
					if !bytes.Equal(got, want) || gotErr != wantErr {
						t.Fatalf("result differs: bytes=%d/%d err=%v/%v", len(got), len(want), gotErr, wantErr)
					}
				})
			}
		}
	}
}

func TestProfileImageBodyReadsBeyondHint(t *testing.T) {
	input := bytes.Repeat([]byte("a"), 1<<20)
	for _, hint := range []int64{1, 1024, int64(len(input))} {
		got, err := readProfileImageBody(bytes.NewReader(input), hint)
		if err != nil || !bytes.Equal(got, input) {
			t.Fatalf("hint=%d truncated input: %v", hint, err)
		}
	}
}

func TestProfileImageBodyLargeInvalidConsumesAndClassifies(t *testing.T) {
	data := bytes.Repeat([]byte("invalid JPEG"), (30<<20)/12)
	for _, hint := range []int64{int64(len(data)), -1} {
		r := httptest.NewRequest(http.MethodPut, "/api/me/profile-image", bytes.NewReader(data))
		r.ContentLength = hint
		r.Header.Set("Content-Type", "image/jpeg")
		ctx := echo.New().NewContext(r, httptest.NewRecorder())
		err := putProfileImageHandler(&HandlerContext{})(ctx)
		var apiErr *APIError
		if !errors.As(err, &apiErr) || apiErr.Status != 400 || apiErr.Code != "invalid_profile_image" || ctx.Get("abuse") != "invalid_image" {
			t.Fatalf("invalid JPEG result changed: %v", err)
		}
		if remaining, err := io.ReadAll(r.Body); err != nil || len(remaining) != 0 {
			t.Fatal("body was not fully consumed")
		}
	}
}

func TestProfileImageBodyHTTPKnownAndChunked(t *testing.T) {
	db, _ := architectureDB(t)
	owner, token := fixtureOwner(t, db)
	runtime := localRuntime(t, db, t.TempDir())
	server := httptest.NewServer(NewRegionServer(&Config{Region: "europe", EdgeKey: architectureTestKey}, runtime))
	defer server.Close()
	// Trailing bytes are permitted by the original JPEG decoder and must be
	// stored unchanged, even when the body is much larger than the pixel data.
	data := append(jpegFixture(t, 16), make([]byte, 30<<20)...)
	for _, chunked := range []bool{false, true} {
		r, err := http.NewRequest(http.MethodPut, server.URL+"/api/me/profile-image", bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		r.Header.Set("Authorization", "Bearer "+token)
		r.Header.Set("Content-Type", "image/jpeg")
		if chunked {
			r.ContentLength = -1
		}
		resp, err := http.DefaultClient.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("chunked=%v: status=%d", chunked, resp.StatusCode)
		}
		var imageID string
		if err := db.Get(&imageID, `SELECT profile_image_id FROM users WHERE id=$1`, owner.ID); err != nil {
			t.Fatal(err)
		}
		got := request(t, http.MethodGet, server.URL+signedImageURL(owner, imageID), "", "", nil, http.StatusOK)
		if !bytes.Equal(got, data) {
			t.Fatalf("chunked=%v: stored image bytes changed", chunked)
		}
	}
}

var benchmarkProfileBody []byte

func BenchmarkProfileImageBodyRead(b *testing.B) {
	for _, size := range []int{32 << 10, 1 << 20, 30 << 20} {
		data := bytes.Repeat([]byte{0xa5}, size)
		for _, preallocate := range []bool{false, true} {
			b.Run(fmt.Sprintf("bytes=%d/preallocate=%t", size, preallocate), func(b *testing.B) {
				b.ReportAllocs()
				b.SetBytes(int64(size))
				b.ResetTimer()
				for range b.N {
					reader := bytes.NewReader(data)
					var err error
					if preallocate {
						benchmarkProfileBody, err = readProfileImageBody(reader, int64(size))
					} else {
						benchmarkProfileBody, err = io.ReadAll(reader)
					}
					if err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}
