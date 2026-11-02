package isureal

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/gob"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
)

const edgeMemoryHeader = "X-Isureal-Memory-Rendering"
const edgeMemoryContentType = "application/vnd.isureal.edge-memory-v1"
const edgeMemoryMaxBytes = 8 << 20

// This internal model includes signing keys and profile image IDs. It must only
// cross the internal link inside the authenticated encryption envelope below.
// Gob deliberately preserves fields excluded from public JSON. Never JSON-encode
// this model or send it directly to an external client.
type edgeMemoryData struct {
	Posts     []PostWithAuthor
	Reactions []RealMojiWithReactor
	Comments  []CommentWithAuthor
	HasMore   bool
	ExpiresAt int64
}

func edgeMemoryRenderingEnabled() bool {
	return os.Getenv("ISUREAL_EDGE_MEMORY_RENDERING") == "true"
}

func isMemoryRequest(r *http.Request) bool {
	return r.Method == http.MethodGet && (r.URL.Path == "/api/memories" || strings.HasPrefix(r.URL.Path, "/api/memories/"))
}

func memoryRequestBinding(r *http.Request) string {
	// Bind the envelope to the request and session without placing credentials
	// in transport metadata or logs.
	session := sha256.Sum256([]byte(r.Header.Get("Cookie") + "\x00" + r.Header.Get("Authorization")))
	return r.Method + "\x00" + r.URL.RequestURI() + "\x00" + string(session[:])
}

func wantsEdgeMemory(r *http.Request, key string) bool {
	return isMemoryRequest(r) && validEdgeMAC(key, "memory-rendering-v1", memoryRequestBinding(r), r.Header.Get(edgeMemoryHeader))
}

func memoryAEAD(key string) (cipher.AEAD, error) {
	if len(key) < 32 {
		return nil, errors.New("invalid edge memory key")
	}
	digest := sha256.Sum256([]byte("isureal:edge-memory:v1\x00" + key))
	block, err := aes.NewCipher(digest[:])
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func sealMemoryData(data edgeMemoryData, key, binding string) ([]byte, error) {
	var plain bytes.Buffer
	if err := gob.NewEncoder(&plain).Encode(data); err != nil {
		return nil, err
	}
	aead, err := memoryAEAD(key)
	if err != nil {
		return nil, err
	}
	if plain.Len()+aead.NonceSize()+aead.Overhead() > edgeMemoryMaxBytes {
		return nil, nil
	}
	nonce := make([]byte, aead.NonceSize(), aead.NonceSize()+plain.Len()+aead.Overhead())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return aead.Seal(nonce, nonce, plain.Bytes(), []byte(binding)), nil
}

func openMemoryData(body []byte, key, binding string) (edgeMemoryData, error) {
	var data edgeMemoryData
	aead, err := memoryAEAD(key)
	if err != nil {
		return data, err
	}
	if len(body) > edgeMemoryMaxBytes || len(body) < aead.NonceSize()+aead.Overhead() {
		return data, errors.New("invalid edge memory envelope")
	}
	plain, err := aead.Open(nil, body[:aead.NonceSize()], body[aead.NonceSize():], []byte(binding))
	if err != nil {
		return data, errors.New("invalid edge memory authentication")
	}
	if err := gob.NewDecoder(bytes.NewReader(plain)).Decode(&data); err != nil {
		return data, errors.New("invalid edge memory data")
	}
	return data, nil
}

func loadMemoryData(ctx context.Context, db sqlx.QueryerContext, userID string, start, end time.Time, beforeAt *time.Time, beforeID string) (edgeMemoryData, error) {
	var data edgeMemoryData
	if (beforeAt == nil) != (beforeID == "") {
		return data, NewAPIError(http.StatusBadRequest, "invalid_pagination")
	}
	posts, err := selectPostsWithAuthor(ctx, db, `
 AND p.author_id=$1 AND w.local_date >= $2 AND w.local_date < $3
 AND ($4::timestamptz IS NULL OR (p.posted_at,p.id)<($4,$5))
 ORDER BY p.posted_at DESC,p.id DESC LIMIT 51`, userID, start, end, beforeAt, beforeID)
	if err != nil {
		return data, err
	}
	data.HasMore = len(posts) > 50
	if data.HasMore {
		posts = posts[:50]
	}
	data.Posts = posts
	data.ExpiresAt = imageURLExpiry(time.Now(), imageURLExpiryBucket)
	if len(posts) == 0 {
		return data, nil
	}
	ids := make([]string, len(posts))
	for i, p := range posts {
		ids[i] = p.ID
	}
	data.Reactions, err = selectCurrentRealMojis(ctx, db, memoryReactionConditions, ids)
	if err != nil {
		return data, err
	}
	data.Comments, err = selectCommentsWithAuthor(ctx, db, memoryCommentConditions, ids)
	return data, err
}

func renderMemoryData(data edgeMemoryData) getMemoriesResponse {
	items := make([]memoryPostResponse, len(data.Posts))
	byID := make(map[string]*memoryPostResponse, len(items))
	for i, post := range data.Posts {
		post.Author.ImageURLExpiry = data.ExpiresAt
		items[i] = memoryPostResponse{createPostResponse: newPostResponse(post.Post, post.Author), RealMojis: []replaceRealMojiResponse{}, Comments: []createCommentResponse{}}
		byID[post.ID] = &items[i]
	}
	for _, reaction := range data.Reactions {
		reaction.Reactor.ImageURLExpiry = data.ExpiresAt
		item := byID[reaction.PostID]
		if item != nil && len(item.RealMojis) < 50 {
			item.RealMojis = append(item.RealMojis, newRealMojiResponse(reaction.RealMoji, reaction.Reactor))
		}
	}
	for _, comment := range data.Comments {
		comment.Author.ImageURLExpiry = data.ExpiresAt
		item := byID[comment.PostID]
		if item != nil && len(item.Comments) < 50 {
			item.Comments = append(item.Comments, newCommentResponse(comment.Comment, comment.Author))
		}
	}
	return getMemoriesResponse{Items: items, HasMore: data.HasMore}
}

func renderEdgeMemoryResponse(resp *http.Response, key string) error {
	if resp.Header.Get("Content-Type") != edgeMemoryContentType {
		return nil
	}
	// A model response to an unmarked/public request must never escape as bytes.
	if resp.StatusCode != http.StatusOK || !wantsEdgeMemory(resp.Request, key) {
		resp.Body.Close()
		return errors.New("unexpected edge memory response")
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, edgeMemoryMaxBytes+1))
	resp.Body.Close()
	if err != nil {
		return err
	}
	data, err := openMemoryData(body, key, memoryRequestBinding(resp.Request))
	if err != nil {
		return err
	}
	var output bytes.Buffer
	if err := json.NewEncoder(&output).Encode(renderMemoryData(data)); err != nil {
		return err
	}
	resp.Body = io.NopCloser(bytes.NewReader(output.Bytes()))
	resp.ContentLength = int64(output.Len())
	resp.Header.Del("Content-Length")
	resp.Header.Set("Content-Type", "application/json")
	resp.Header.Set("Cache-Control", "no-store")
	return nil
}
