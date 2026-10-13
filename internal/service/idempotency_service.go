package service

import (
	"context"
	"time"

	"github.com/ledger/transaction-engine/internal/domain"
	"github.com/ledger/transaction-engine/internal/repository/postgres"
	"github.com/ledger/transaction-engine/internal/repository/redis"
)

type IdempotencyService struct {
	pgRepo    *postgres.IdempotencyRepository
	redisLock *redis.LockService
	ttl       time.Duration
}

func NewIdempotencyService(
	pgRepo *postgres.IdempotencyRepository,
	redisLock *redis.LockService,
	ttl time.Duration,
) *IdempotencyService {
	return &IdempotencyService{
		pgRepo:    pgRepo,
		redisLock: redisLock,
		ttl:       ttl,
	}
}

type IdempotencyCheckResult struct {
	IsReplay     bool
	StatusCode   int
	ResponseBody []byte
	Unlock       func()
}

// PrepareExecution checks if the request is duplicate, acquires locks, and reserves the key
func (s *IdempotencyService) PrepareExecution(ctx context.Context, key string, requestPayload []byte) (*IdempotencyCheckResult, error) {
	if key == "" {
		return &IdempotencyCheckResult{IsReplay: false, Unlock: func() {}}, nil
	}

	reqHash := domain.HashPayload(requestPayload)

	// 1. Check Redis cache for fast replay
	cached, err := s.redisLock.GetCachedIdempotencyResponse(ctx, key)
	if err == nil && cached != nil {
		if cached.Hash != reqHash {
			return nil, domain.ErrIdempotencyKeyMismatch
		}
		return &IdempotencyCheckResult{
			IsReplay:     true,
			StatusCode:   cached.StatusCode,
			ResponseBody: cached.Body,
			Unlock:       func() {},
		}, nil
	}

	// 2. Check Postgres table
	record, err := s.pgRepo.GetRecord(ctx, key)
	if err != nil {
		return nil, err
	}
	if record != nil {
		if !record.Matches(reqHash) {
			return nil, domain.ErrIdempotencyKeyMismatch
		}

		if record.Status == domain.IdempotencyStatusCompleted {
			// Backfill Redis cache
			_ = s.redisLock.CacheIdempotencyResponse(ctx, key, reqHash, record.ResponseCode, record.ResponseBody, s.ttl)
			return &IdempotencyCheckResult{
				IsReplay:     true,
				StatusCode:   record.ResponseCode,
				ResponseBody: record.ResponseBody,
				Unlock:       func() {},
			}, nil
		}

		if record.Status == domain.IdempotencyStatusProcessing {
			return nil, domain.ErrIdempotencyKeyConflict
		}
	}

	// 3. Acquire distributed lock in Redis for concurrency protection
	unlock, acquired, err := s.redisLock.AcquireLock(ctx, key, 10*time.Second)
	if err != nil {
		return nil, err
	}
	if !acquired {
		return nil, domain.ErrIdempotencyKeyConflict
	}

	// 4. Mark key as PROCESSING in database
	if err := s.pgRepo.CreateProcessingRecord(ctx, key, reqHash, s.ttl); err != nil {
		unlock()
		// If another worker raced to insert the row
		return nil, domain.ErrIdempotencyKeyConflict
	}

	return &IdempotencyCheckResult{
		IsReplay: false,
		Unlock:   unlock,
	}, nil
}

// CompleteExecution marks the idempotency key as completed and populates caches
func (s *IdempotencyService) CompleteExecution(ctx context.Context, key string, reqPayload []byte, code int, respBody []byte) error {
	if key == "" {
		return nil
	}

	reqHash := domain.HashPayload(reqPayload)

	// Update DB record
	if err := s.pgRepo.CompleteRecord(ctx, key, code, respBody); err != nil {
		return err
	}

	// Update Redis cache
	_ = s.redisLock.CacheIdempotencyResponse(ctx, key, reqHash, code, respBody, s.ttl)
	return nil
}

// FailExecution cleans up idempotency reservation when execution fails before DB commit
func (s *IdempotencyService) FailExecution(ctx context.Context, key string) {
	if key == "" {
		return
	}
	_ = s.pgRepo.DeleteRecord(ctx, key)
}
