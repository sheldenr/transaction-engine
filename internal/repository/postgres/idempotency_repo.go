package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/ledger/transaction-engine/internal/domain"
)

type IdempotencyRepository struct {
	db *DB
}

func NewIdempotencyRepository(db *DB) *IdempotencyRepository {
	return &IdempotencyRepository{db: db}
}

func (r *IdempotencyRepository) GetRecord(ctx context.Context, key string) (*domain.IdempotencyRecord, error) {
	query := `
		SELECT key, request_hash, status, response_code, response_body, created_at, updated_at, expires_at
		FROM idempotency_keys
		WHERE key = $1
	`
	var rec domain.IdempotencyRecord
	var status string
	var code *int
	var body []byte

	err := r.db.Pool.QueryRow(ctx, query, key).Scan(
		&rec.Key,
		&rec.RequestHash,
		&status,
		&code,
		&body,
		&rec.CreatedAt,
		&rec.UpdatedAt,
		&rec.ExpiresAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil // Key not found is normal for first request
		}
		return nil, fmt.Errorf("failed to query idempotency key: %w", err)
	}

	rec.Status = domain.IdempotencyStatus(status)
	if code != nil {
		rec.ResponseCode = *code
	}
	rec.ResponseBody = body
	return &rec, nil
}

func (r *IdempotencyRepository) CreateProcessingRecord(ctx context.Context, key, requestHash string, ttl time.Duration) error {
	now := time.Now().UTC()
	expiresAt := now.Add(ttl)

	query := `
		INSERT INTO idempotency_keys (key, request_hash, status, created_at, updated_at, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := r.db.Pool.Exec(ctx, query, key, requestHash, string(domain.IdempotencyStatusProcessing), now, now, expiresAt)
	if err != nil {
		return fmt.Errorf("failed to insert processing idempotency record: %w", err)
	}
	return nil
}

func (r *IdempotencyRepository) CompleteRecord(ctx context.Context, key string, statusCode int, responseBody []byte) error {
	query := `
		UPDATE idempotency_keys
		SET status = $1, response_code = $2, response_body = $3, updated_at = NOW()
		WHERE key = $4
	`
	_, err := r.db.Pool.Exec(ctx, query, string(domain.IdempotencyStatusCompleted), statusCode, responseBody, key)
	if err != nil {
		return fmt.Errorf("failed to complete idempotency record: %w", err)
	}
	return nil
}

func (r *IdempotencyRepository) DeleteRecord(ctx context.Context, key string) error {
	query := `DELETE FROM idempotency_keys WHERE key = $1`
	_, err := r.db.Pool.Exec(ctx, query, key)
	return err
}
