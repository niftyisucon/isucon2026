package isureal

import (
	"os"
	"strings"
	"sync"
	"sync/atomic"
)

// sessionCache keeps active login sessions in the shared core process. Every
// login_sessions and users.status/region write on asia1 goes through this
// process, so those writes can invalidate it directly. PostgreSQL stays the
// source of truth: a miss or a restart only falls back to the existing query.
//
// A nil cache is valid and disabled. Edges and per-region processes must leave it
// nil because they cannot observe writes made by another process.
type sessionCache struct {
	mu       sync.RWMutex
	clock    uint64
	resetAt  uint64
	sessions map[string]Session
	tokens   map[string]map[string]struct{}
	// Clock of the latest invalidation per user. A lookup that started before it
	// may have read the old row, so its result must not be stored.
	invalidatedAt map[string]uint64
}

var sessionCacheHits, sessionCacheMisses atomic.Uint64

func newSessionCache() *sessionCache {
	return &sessionCache{
		sessions:      map[string]Session{},
		tokens:        map[string]map[string]struct{}{},
		invalidatedAt: map[string]uint64{},
	}
}

// Only the multi-region core is the single writer. ISUREAL_SESSION_CACHE=false
// returns it to a DB lookup per request without any other change.
func sessionCacheEnabled(config *Config) bool {
	return config.EdgeUpstream == "" && strings.Contains(config.ListenRegions, ",") &&
		os.Getenv("ISUREAL_SESSION_CACHE") != "false"
}

func (c *sessionCache) get(tokenHash string) *Session {
	if c == nil {
		return nil
	}
	c.mu.RLock()
	session, ok := c.sessions[tokenHash]
	c.mu.RUnlock()
	if !ok {
		sessionCacheMisses.Add(1)
		return nil
	}
	sessionCacheHits.Add(1)
	return &session
}

// start must be called before reading PostgreSQL. Its value is passed to put.
func (c *sessionCache) start() uint64 {
	if c == nil {
		return 0
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.clock
}

func (c *sessionCache) put(session Session, started uint64) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.resetAt > started || c.invalidatedAt[session.UserID] > started {
		return
	}
	session.Token = ""
	c.sessions[session.TokenHash] = session
	tokens := c.tokens[session.UserID]
	if tokens == nil {
		tokens = map[string]struct{}{}
		c.tokens[session.UserID] = tokens
	}
	tokens[session.TokenHash] = struct{}{}
}

// invalidateUser must run after the commit attempt, including a failed one: a
// commit error does not prove that the server rolled the transaction back.
func (c *sessionCache) invalidateUser(userID string) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.clock++
	c.invalidatedAt[userID] = c.clock
	for tokenHash := range c.tokens[userID] {
		delete(c.sessions, tokenHash)
	}
	delete(c.tokens, userID)
}

func (c *sessionCache) reset() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.clock++
	c.resetAt = c.clock
	c.sessions = map[string]Session{}
	c.tokens = map[string]map[string]struct{}{}
	c.invalidatedAt = map[string]uint64{}
}

func sessionCacheStats() map[string]uint64 {
	return map[string]uint64{"hits": sessionCacheHits.Load(), "misses": sessionCacheMisses.Load()}
}
