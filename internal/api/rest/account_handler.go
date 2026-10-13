package rest

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/ledger/transaction-engine/internal/api/dto"
	"github.com/ledger/transaction-engine/internal/domain"
	"github.com/ledger/transaction-engine/internal/service"
)

type AccountHandler struct {
	ledgerService *service.LedgerService
}

func NewAccountHandler(ledgerService *service.LedgerService) *AccountHandler {
	return &AccountHandler{ledgerService: ledgerService}
}

func (h *AccountHandler) CreateAccount(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "Invalid request payload: "+err.Error(), "INVALID_PAYLOAD")
		return
	}

	acc, err := h.ledgerService.CreateAccount(r.Context(), req.Name, req.Currency, req.InitialBalance)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrInvalidCurrency):
			ErrorJSON(w, http.StatusBadRequest, err.Error(), "INVALID_CURRENCY")
		case errors.Is(err, domain.ErrNegativeAmount):
			ErrorJSON(w, http.StatusBadRequest, err.Error(), "NEGATIVE_AMOUNT")
		default:
			ErrorJSON(w, http.StatusBadRequest, err.Error(), "VALIDATION_FAILED")
		}
		return
	}

	JSON(w, http.StatusCreated, acc)
}

func (h *AccountHandler) GetAccount(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		ErrorJSON(w, http.StatusBadRequest, "Invalid account ID UUID format", "INVALID_UUID")
		return
	}

	acc, err := h.ledgerService.GetAccount(r.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrAccountNotFound) {
			ErrorJSON(w, http.StatusNotFound, "Account not found", "ACCOUNT_NOT_FOUND")
			return
		}
		ErrorJSON(w, http.StatusInternalServerError, "Internal database error", "INTERNAL_ERROR")
		return
	}

	JSON(w, http.StatusOK, acc)
}

func (h *AccountHandler) ListAccounts(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	limit := 20
	offset := 0
	if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
		limit = l
	}
	if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
		offset = o
	}

	accounts, err := h.ledgerService.ListAccounts(r.Context(), limit, offset)
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "Failed to list accounts", "INTERNAL_ERROR")
		return
	}

	if accounts == nil {
		accounts = []domain.Account{}
	}

	JSON(w, http.StatusOK, map[string]interface{}{
		"accounts": accounts,
		"limit":    limit,
		"offset":   offset,
	})
}

func (h *AccountHandler) GetAccountEntries(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		ErrorJSON(w, http.StatusBadRequest, "Invalid account ID UUID format", "INVALID_UUID")
		return
	}

	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	limit := 50
	offset := 0
	if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
		limit = l
	}
	if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
		offset = o
	}

	entries, err := h.ledgerService.GetAccountHistory(r.Context(), id, limit, offset)
	if err != nil {
		if errors.Is(err, domain.ErrAccountNotFound) {
			ErrorJSON(w, http.StatusNotFound, "Account not found", "ACCOUNT_NOT_FOUND")
			return
		}
		ErrorJSON(w, http.StatusInternalServerError, "Failed to fetch account statement", "INTERNAL_ERROR")
		return
	}

	if entries == nil {
		entries = []domain.Entry{}
	}

	JSON(w, http.StatusOK, map[string]interface{}{
		"account_id": id,
		"entries":    entries,
		"limit":      limit,
		"offset":     offset,
	})
}
