package metrics

import (
	"context"
	"math/rand"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// Tracker wraps a Redis client and provides helpers for recording request metrics.
type Tracker struct {
	client     *redis.Client
	userAgents []string
	mu         sync.Mutex
}

// NewTracker constructs a tracker using the provided Redis options. If opts is nil,
// it falls back to the default redis.Options.
func NewTracker(opts *redis.Options) *Tracker {
	if opts == nil {
		opts = &redis.Options{}
	}

	return &Tracker{
		client:     redis.NewClient(opts),
		userAgents: make([]string, 8192),
	}
}

// Client exposes the underlying Redis client for direct use when needed.
func (t *Tracker) Client() *redis.Client {
	return t.client
}

// RecordRequest tracks metadata for a caller IP + user agent combination.
func (t *Tracker) RecordRequest(ctx context.Context, ip string, userAgent string) error {
	if userAgent == "" {
		userAgent = "unknown"
	}
	if decoded, err := url.QueryUnescape(userAgent); err == nil {
		userAgent = decoded
	}

	parts := strings.FieldsFunc(userAgent, func(r rune) bool {
		return r == '/' || r == ' ' || r == ';'
	})
	if len(parts) > 0 {
		userAgent = parts[0]
	}

	idx := rand.Intn(len(t.userAgents))
	t.mu.Lock()
	t.userAgents[idx] = userAgent
	t.mu.Unlock()

	now := time.Now().UTC()
	hour := strconv.FormatInt(now.Unix()/3600, 10)
	day := strconv.FormatInt(now.Unix()/86400, 10)

	pipe := t.client.Pipeline()

	pipe.Incr(ctx, "h:"+hour)
	pipe.Incr(ctx, "d:"+day)

	if strings.Contains(ip, ":") {
		pipe.Incr(ctx, "d6:"+day)
	} else {
		pipe.Incr(ctx, "d4:"+day)
	}

	pipe.PFAdd(ctx, "ph:"+hour, ip)
	pipe.PFAdd(ctx, "pd:"+day, ip)

	pipe.Expire(ctx, "h:"+hour, 31*24*time.Hour)
	pipe.Expire(ctx, "d:"+day, 366*24*time.Hour)
	pipe.Expire(ctx, "ph:"+hour, 31*24*time.Hour)
	pipe.Expire(ctx, "pd:"+day, 366*24*time.Hour)
	pipe.Expire(ctx, "d4:"+day, 366*24*time.Hour)
	pipe.Expire(ctx, "d6:"+day, 366*24*time.Hour)

	_, err := pipe.Exec(ctx)
	return err
}

// UserAgentCounts returns the observed user agent sample as aggregated counts.
func (t *Tracker) UserAgentCounts() map[string]int {
	counts := make(map[string]int)

	t.mu.Lock()
	defer t.mu.Unlock()

	for _, ua := range t.userAgents {
		if ua != "" {
			counts[ua]++
		}
	}

	return counts
}

// GetInt64 normalizes Redis command results to int64, handling Nil responses.
func GetInt64(result redis.Cmder) int64 {
	switch v := result.(type) {
	case *redis.StringCmd:
		val, err := v.Int64()
		if err == redis.Nil {
			return 0
		}
		if err != nil {
			return 0
		}
		return val
	case *redis.IntCmd:
		return v.Val()
	default:
		return 0
	}
}
