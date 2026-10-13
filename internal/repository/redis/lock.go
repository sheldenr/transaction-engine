package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"
)

var releaseLockScript = `
if redis.call("get", KEYS[1]) == ARGV[1] then
    return redis.call("del", KEYS[1])
else
    return 0
end
`

type LockService struct {
	client *Client
}

func NewLockService(client *Client) *LockService {
	return &LockService{client: client}
}

type CachedResponse struct {
	StatusCode int    `json:"status_code"`
	Body       []byte `json:"body"`
	Hash       string `json:"hash"`
}

// AcquireLock attempts to acquire a distributed lock with a timeout
func (s *LockService) AcquireLock(ctx context.Context, resourceKey string, ttl time.Duration) (func(), bool, error) {
	if s.client == nil || s.client.Rdb == nil {
		// Redis disabled / optional fallback
		return func() {}, true, nil
	}

	lockKey := fmt.Sprintf("lock:%s", resourceKey)
	lockVal := uuid.New().String()

	success, err := s.client.Rdb.SetNX(ctx, lockKey, lockVal, ttl).Result()
	if err != nil {
		return nil, false, fmt.Errorf("failed to acquire redis lock: %w", err)
	}
	if !success {
		return nil, false, nil
	}

	unlockFn := func() {
		ctxTimeout, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = s.client.Rdb.Eval(ctxTimeout, releaseLockScript, []string{lockKey}, lockVal).Err()
	}

	return unlockFn, true, nil
}

// CacheIdempotencyResponse stores the completed response in Redis for fast caching
func (s *LockService) CacheIdempotencyResponse(ctx context.Context, key, hash string, code int, body []byte, ttl time.Duration) error {
	if s.client == nil || s.client.Rdb == nil {
		return nil
	}

	cacheKey := fmt.Sprintf("idempotency:%s", key)
	payload := CachedResponse{
		StatusCode: code,
		Body:       body,
		Hash:       hash,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	return s.client.Rdb.Set(ctx, cacheKey, data, ttl).Err()
}

// GetCachedIdempotencyResponse retrieves a previously cached idempotency response
func (s *LockService) GetCachedIdempotencyResponse(ctx context.Context, key string) (*CachedResponse, error) {
	if s.client == nil || s.client.Rdb == nil {
		return nil, nil
	}

	cacheKey := fmt.Sprintf("idempotency:%s", key)
	val, err := s.client.Rdb.Get(ctx, cacheKey).Bytes()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			return nil, nil
		}
		return nil, err
	}

	var res CachedResponse
	if err := json.Unmarshal(val, &res); err != nil {
		return nil, err
	}

	return &res, nil
}
