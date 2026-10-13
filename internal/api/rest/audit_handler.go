package rest

import (
	"context"
	"net/http"
	"time"

	"github.com/ledger/transaction-engine/internal/repository/postgres"
	"github.com/ledger/transaction-engine/internal/repository/redis"
	"github.com/ledger/transaction-engine/internal/service"
)

type AuditHandler struct {
	ledgerService *service.LedgerService
	db            *postgres.DB
	redisClient   *redis.Client
}

func NewAuditHandler(ledgerService *service.LedgerService, db *postgres.DB, redisClient *redis.Client) *AuditHandler {
	return &AuditHandler{
		ledgerService: ledgerService,
		db:            db,
		redisClient:   redisClient,
	}
}

func (h *AuditHandler) AuditLedger(w http.ResponseWriter, r *http.Request) {
	result, err := h.ledgerService.AuditLedger(r.Context())
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "Audit failed: "+err.Error(), "AUDIT_FAILED")
		return
	}

	statusCode := http.StatusOK
	if !result.IsLedgerBalanced || len(result.UnbalancedTransactionIDs) > 0 {
		statusCode = http.StatusConflict
	}

	JSON(w, statusCode, result)
}

func (h *AuditHandler) Healthz(w http.ResponseWriter, r *http.Request) {
	JSON(w, http.StatusOK, map[string]string{
		"status": "UP",
	})
}

func (h *AuditHandler) Readyz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	checks := make(map[string]string)
	isReady := true

	if err := h.db.Pool.Ping(ctx); err != nil {
		checks["postgres"] = "DOWN: " + err.Error()
		isReady = false
	} else {
		checks["postgres"] = "UP"
	}

	if h.redisClient != nil && h.redisClient.Rdb != nil {
		if err := h.redisClient.Rdb.Ping(ctx).Err(); err != nil {
			checks["redis"] = "DOWN: " + err.Error()
			isReady = false
		} else {
			checks["redis"] = "UP"
		}
	} else {
		checks["redis"] = "SKIPPED"
	}

	statusCode := http.StatusOK
	if !isReady {
		statusCode = http.StatusServiceUnavailable
	}

	JSON(w, statusCode, map[string]interface{}{
		"ready":  isReady,
		"checks": checks,
	})
}
