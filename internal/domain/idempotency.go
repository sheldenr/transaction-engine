package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"time"
)

type IdempotencyStatus string

const (
	IdempotencyStatusProcessing IdempotencyStatus = "PROCESSING"
	IdempotencyStatusCompleted  IdempotencyStatus = "COMPLETED"
	IdempotencyStatusFailed     IdempotencyStatus = "FAILED"
)

type IdempotencyRecord struct {
	Key          string            `json:"key"`
	RequestHash  string            `json:"request_hash"`
	Status       IdempotencyStatus `json:"status"`
	ResponseCode int               `json:"response_code"`
	ResponseBody []byte            `json:"response_body"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
	ExpiresAt    time.Time         `json:"expires_at"`
}

func HashPayload(payload []byte) string {
	h := sha256.Sum256(payload)
	return hex.EncodeToString(h[:])
}

func (r *IdempotencyRecord) Matches(hash string) bool {
	return r.RequestHash == hash
}

func (r *IdempotencyRecord) IsExpired() bool {
	return time.Now().UTC().After(r.ExpiresAt)
}
