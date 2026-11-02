package isureal

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/labstack/echo/v5"
	"go.opentelemetry.io/otel/propagation"
)

type edgeCachedContext struct{}
type edgeCacheSnapshot struct {
	image      *cachedImage
	generation uint64
	upload     []byte
}

const edgeMaxBody = 32 << 20

func newEdgeProxy(config *Config, cache *ImageCache) echo.HandlerFunc {
	uploadImageCache := os.Getenv("ISUREAL_EDGE_UPLOAD_IMAGE_CACHE") != "false"
	target, err := url.Parse(config.EdgeUpstream)
	if err != nil || target.Scheme != "http" || target.Host == "" || target.Path != "" || target.RawQuery != "" || target.User != nil {
		panic("invalid edge upstream")
	}
	if len(config.EdgeKey) < 32 {
		panic("ISUREAL_EDGE_KEY must contain at least 32 bytes")
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.Transport = &http.Transport{Proxy: nil, MaxIdleConns: 128, MaxIdleConnsPerHost: 64, IdleConnTimeout: 30 * time.Second, ResponseHeaderTimeout: 90 * time.Second}
	proxy.FlushInterval = -1
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		slog.Warn("edge proxy upstream error", "method", r.Method, "path", r.URL.Path, "error", err)
		http.Error(w, "upstream unavailable", http.StatusBadGateway)
	}
	proxy.ModifyResponse = func(resp *http.Response) error {
		if err := renderEdgeMemoryResponse(resp, config.EdgeKey); err != nil {
			return err
		}
		if resp.Request.Method == http.MethodPost && resp.Request.URL.Path == "/api/posts" && resp.StatusCode == http.StatusCreated && resp.Header.Get("Content-Encoding") == "" && !resp.Uncompressed {
			snapshot, _ := resp.Request.Context().Value(edgeCachedContext{}).(edgeCacheSnapshot)
			if snapshot.upload != nil {
				cacheUploadedPostImage(resp, cache, snapshot)
			}
		}
		id := edgeImageID(resp.Request)
		if id != "" {
			snapshot, _ := resp.Request.Context().Value(edgeCachedContext{}).(edgeCacheSnapshot)
			cached := snapshot.image
			if resp.StatusCode == http.StatusNoContent && cached != nil && resp.Request.Header.Get(cachedImageHeader) != "" {
				resp.Body.Close()
				resp.StatusCode = http.StatusOK
				resp.Status = "200 OK"
				resp.Body = io.NopCloser(bytes.NewReader(cached.Data))
				resp.ContentLength = int64(len(cached.Data))
				resp.Header.Set("Content-Type", cached.MediaType)
				resp.Header.Del("Content-Length")
			} else if resp.StatusCode == http.StatusOK {
				data, err := io.ReadAll(io.LimitReader(resp.Body, edgeMaxBody+1))
				if err != nil {
					return err
				}
				resp.Body.Close()
				if len(data) > edgeMaxBody {
					return echo.ErrBadGateway
				}
				resp.Body = io.NopCloser(bytes.NewReader(data))
				resp.ContentLength = int64(len(data))
				cache.PutGeneration(id, &cachedImage{MediaType: resp.Header.Get("Content-Type"), Data: data}, snapshot.generation)
			}
		}
		if resp.Request.URL.Path == "/api/initialize" && resp.StatusCode == http.StatusOK {
			cache.Reset()
			validatedJPEGs.reset()
			signedImageURLs.reset()
		}
		return nil
	}
	return func(c *echo.Context) error {
		r := c.Request()
		r.Header.Del(validationHeader)
		r.Header.Del(cachedImageHeader)
		r.Header.Del(edgeMemoryHeader)
		if edgeMemoryRenderingEnabled() && isMemoryRequest(r) {
			r.Header.Set(edgeMemoryHeader, edgeMAC(config.EdgeKey, "memory-rendering-v1", memoryRequestBinding(r)))
		}
		cached, generation := cache.GetGeneration(edgeImageID(r))
		snapshot := edgeCacheSnapshot{image: cached, generation: generation}
		if id := edgeImageID(r); id != "" && cached != nil {
			r.Header.Set(cachedImageHeader, edgeMAC(config.EdgeKey, "cached-image", r.URL.RequestURI()))
		}
		// Invalid images are forwarded without proof so authentication, field
		// validation and error/abuse ordering remain identical to the original API.
		if limit := edgeUploadLimit(r); limit != 0 {
			if r.ContentLength > edgeMaxBody {
				return c.NoContent(http.StatusRequestEntityTooLarge)
			}
			raw, err := io.ReadAll(io.LimitReader(r.Body, edgeMaxBody+1))
			if err != nil {
				return err
			}
			r.Body.Close()
			if len(raw) > edgeMaxBody {
				return c.NoContent(http.StatusRequestEntityTooLarge)
			}
			r.Body = io.NopCloser(bytes.NewReader(raw))
			data := raw
			if r.URL.Path != "/api/me/profile-image" {
				clone := r.Clone(r.Context())
				clone.Body = io.NopCloser(bytes.NewReader(raw))
				if err := clone.ParseMultipartForm(edgeMaxBody); err == nil {
					defer clone.MultipartForm.RemoveAll()
					f, _, err := clone.FormFile("image")
					if err == nil {
						data, err = io.ReadAll(f)
						f.Close()
						if err != nil {
							data = nil
						}
					} else {
						data = nil
					}
				} else {
					data = nil
				}
			}
			if data != nil && decodeJPEG(data, limit) == nil {
				r.Header.Set(validationHeader, edgeMAC(config.EdgeKey, "jpeg", imageValidationValue(data, limit)))
				if uploadImageCache && r.Method == http.MethodPost && r.URL.Path == "/api/posts" {
					// createPostHandler persists these exact multipart bytes without re-encoding.
					snapshot.upload = data
				}
			}
		}
		r = r.WithContext(context.WithValue(r.Context(), edgeCachedContext{}, snapshot))
		// Keep edge and core in the same sampled trace. Do not record URL IDs,
		// request bodies, credentials or the internal validation headers.
		propagation.TraceContext{}.Inject(r.Context(), propagation.HeaderCarrier(r.Header))
		proxy.ServeHTTP(c.Response(), r)
		return nil
	}
}

func cacheUploadedPostImage(resp *http.Response, cache *ImageCache, snapshot edgeCacheSnapshot) {
	body := resp.Body
	data, err := io.ReadAll(io.LimitReader(body, edgeMaxBody+1))
	// Replay the original bytes, including any unread suffix if inspection failed
	// or exceeded the limit. Cache population must never reject a successful post.
	resp.Body = struct {
		io.Reader
		io.Closer
	}{io.MultiReader(bytes.NewReader(data), body), body}
	if err != nil || len(data) > edgeMaxBody {
		return
	}
	var posted struct {
		ImageURL string `json:"imageUrl"`
	}
	if json.Unmarshal(data, &posted) != nil {
		return
	}
	u, err := url.Parse(posted.ImageURL)
	if err != nil || u.IsAbs() || u.Host != "" || u.Fragment != "" {
		return
	}
	if id := edgeImageID(&http.Request{Method: http.MethodGet, URL: u}); id != "" {
		cache.PutGeneration(id, &cachedImage{MediaType: "image/jpeg", Data: snapshot.upload}, snapshot.generation)
	}
}

func edgeImageID(r *http.Request) string {
	if r.Method != http.MethodGet || !strings.HasPrefix(r.URL.Path, "/api/images/") {
		return ""
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/images/")
	if id == "" || strings.Contains(id, "/") {
		return ""
	}
	return id
}
func edgeUploadLimit(r *http.Request) int {
	if r.Method == http.MethodPut && r.URL.Path == "/api/me/profile-image" && r.Header.Get("Content-Type") == "image/jpeg" {
		return 512
	}
	if r.Method == http.MethodPost {
		if r.URL.Path == "/api/posts" {
			return 4096
		}
		if strings.HasPrefix(r.URL.Path, "/api/posts/") && strings.HasSuffix(r.URL.Path, "/realmojis") {
			return 512
		}
	}
	return 0
}
