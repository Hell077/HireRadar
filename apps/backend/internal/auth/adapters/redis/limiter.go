package redis

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	redis "github.com/redis/go-redis/v9"
)

var limitScript = redis.NewScript(`
local count = redis.call('INCR', KEYS[1])
if count == 1 then redis.call('PEXPIRE', KEYS[1], ARGV[1]) end
return count
`)

type Limiter struct{ client *redis.Client }

func NewLimiter(client *redis.Client) *Limiter { return &Limiter{client: client} }

func (l *Limiter) Allow(ctx context.Context, route, clientIP string, limit int64, window time.Duration) (bool, error) {
	if clientIP == "" {
		clientIP = "unknown"
	}
	digest := sha256.Sum256([]byte(clientIP))
	key := "auth:rate:" + route + ":" + hex.EncodeToString(digest[:])
	count, err := limitScript.Run(ctx, l.client, []string{key}, window.Milliseconds()).Int64()
	if err != nil {
		return false, fmt.Errorf("check auth rate limit: %w", err)
	}
	return count <= limit, nil
}
