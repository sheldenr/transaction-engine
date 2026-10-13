package rest

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/ledger/transaction-engine/internal/api/dto"
	"github.com/ledger/transaction-engine/internal/domain"
	"github.com/ledger/transaction-engine/internal/service"
)

type TransactionHandler struct {
	ledgerService *service.LedgerService
}

func NewTransactionHandler(ledgerService *service.LedgerService) *TransactionHandler {
	return &TransactionHandler{ledgerService: ledgerService}
}

func (h *TransactionHandler) CreateTransaction(w http.ResponseWriter, r *http.Request) {
	idempotencyKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		ErrorJSON(w, http.StatusBadRequest, "Failed to read request body", "INVALID_BODY")
		return
	}
	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	var req dto.CreateTransactionRequest
	if err := json.Unmarshal(bodyBytes, &req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "Invalid JSON payload: "+err.Error(), "INVALID_JSON")
		return
	}

	entriesDraft := make([]domain.EntryDraft, 0, len(req.Entries))
	for _, e := range req.Entries {
		dir, err := domain.ParseEntryDirection(e.Direction)
		if err != nil {
			ErrorJSON(w, http.StatusBadRequest, "Invalid entry direction: "+e.Direction, "INVALID_DIRECTION")
			return
		}
		entriesDraft = append(entriesDraft, domain.EntryDraft{
			AccountID: e.AccountID,
			Amount:    e.Amount,
			Direction: dir,
		})
	}

	draft := &domain.TransactionDraft{
		Reference:   req.Reference,
		Description: req.Description,
		Entries:     entriesDraft,
	}

	output, err := h.ledgerService.ExecuteTransaction(r.Context(), draft, idempotencyKey, bodyBytes)
	if err != nil {
		h.handleTransactionError(w, err)
		return
	}

	if output.IsReplay {
		w.Header().Set("X-Idempotent-Replay", "true")
	}

	JSON(w, output.StatusCode, output.Transaction)
}

func (h *TransactionHandler) CreateTransfer(w http.ResponseWriter, r *http.Request) {
	idempotencyKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		ErrorJSON(w, http.StatusBadRequest, "Failed to read request body", "INVALID_BODY")
		return
	}
	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	var req dto.CreateTransferRequest
	if err := json.Unmarshal(bodyBytes, &req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "Invalid JSON payload: "+err.Error(), "INVALID_JSON")
		return
	}

	output, err := h.ledgerService.TransferFunds(
		r.Context(),
		req.SourceAccountID,
		req.DestinationAccountID,
		req.Amount,
		req.Reference,
		req.Description,
		idempotencyKey,
		bodyBytes,
	)
	if err != nil {
		h.handleTransactionError(w, err)
		return
	}

	if output.IsReplay {
		w.Header().Set("X-Idempotent-Replay", "true")
	}

	JSON(w, output.StatusCode, output.Transaction)
}

func (h *TransactionHandler) GetTransaction(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		ErrorJSON(w, http.StatusBadRequest, "Invalid transaction UUID format", "INVALID_UUID")
		return
	}

	txn, err := h.ledgerService.GetTransaction(r.Context(), id)
	if err != nil {
		ErrorJSON(w, http.StatusNotFound, "Transaction not found", "NOT_FOUND")
		return
	}

	JSON(w, http.StatusOK, txn)
}

func (h *TransactionHandler) handleTransactionError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrIdempotencyKeyConflict):
		ErrorJSON(w, http.StatusConflict, err.Error(), "IDEMPOTENCY_CONFLICT")
	case errors.Is(err, domain.ErrIdempotencyKeyMismatch):
		ErrorJSON(w, http.StatusUnprocessableEntity, err.Error(), "IDEMPOTENCY_PAYLOAD_MISMATCH")
	case errors.Is(err, domain.ErrInsufficientFunds):
		ErrorJSON(w, http.StatusUnprocessableEntity, err.Error(), "INSUFFICIENT_FUNDS")
	case errors.Is(err, domain.ErrUnbalancedLedger):
		ErrorJSON(w, http.StatusUnprocessableEntity, err.Error(), "UNBALANCED_LEDGER")
	case errors.Is(err, domain.ErrCurrencyMismatch):
		ErrorJSON(w, http.StatusUnprocessableEntity, err.Error(), "CURRENCY_MISMATCH")
	case errors.Is(err, domain.ErrAccountNotFound):
		ErrorJSON(w, http.StatusNotFound, err.Error(), "ACCOUNT_NOT_FOUND")
	case errors.Is(err, domain.ErrAccountInactive):
		ErrorJSON(w, http.StatusBadRequest, err.Error(), "ACCOUNT_INACTIVE")
	case errors.Is(err, domain.ErrSameAccountTransfer):
		ErrorJSON(w, http.StatusBadRequest, err.Error(), "SAME_ACCOUNT_TRANSFER")
	case errors.Is(err, domain.ErrTooFewEntries),
		errors.Is(err, domain.ErrNegativeAmount),
		errors.Is(err, domain.ErrZeroAmount),
		errors.Is(err, domain.ErrInvalidUUID),
		errors.Is(err, domain.ErrInvalidDirection):
		ErrorJSON(w, http.StatusBadRequest, err.Error(), "INVALID_REQUEST")
	default:
		ErrorJSON(w, http.StatusInternalServerError, "Internal server error: "+err.Error(), "INTERNAL_ERROR")
	}
}
