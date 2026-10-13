package rest_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/ledger/transaction-engine/internal/api/dto"
	"github.com/ledger/transaction-engine/internal/api/rest"
	"github.com/stretchr/testify/assert"
)

func TestHealthCheckEndpoints(t *testing.T) {
	// Simple test verifying healthz returns 200 UP
	auditHandler := rest.NewAuditHandler(nil, nil, nil)

	req, _ := http.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()

	auditHandler.Healthz(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp map[string]string
	err := json.Unmarshal(rr.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "UP", resp["status"])
}

func TestValidationErrors(t *testing.T) {
	t.Run("Create account with invalid payload", func(t *testing.T) {
		handler := rest.NewAccountHandler(nil)
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/accounts", bytes.NewBufferString("{invalid-json"))
		rr := httptest.NewRecorder()

		handler.CreateAccount(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		var errResp rest.ErrorResponse
		err := json.Unmarshal(rr.Body.Bytes(), &errResp)
		assert.NoError(t, err)
		assert.Equal(t, "INVALID_PAYLOAD", errResp.Code)
	})

	t.Run("Get account with invalid UUID", func(t *testing.T) {
		handler := rest.NewAccountHandler(nil)
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/accounts/not-a-uuid", nil)
		rr := httptest.NewRecorder()

		handler.GetAccount(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("Create transaction with invalid direction", func(t *testing.T) {
		handler := rest.NewTransactionHandler(nil)
		reqBody := dto.CreateTransactionRequest{
			Reference:   "REF-1",
			Description: "Invalid direction test",
			Entries: []dto.CreateEntryRequest{
				{AccountID: uuid.New(), Amount: 100, Direction: "INVALID_DIR"},
				{AccountID: uuid.New(), Amount: 100, Direction: "CREDIT"},
			},
		}
		data, _ := json.Marshal(reqBody)
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/transactions", bytes.NewBuffer(data))
		rr := httptest.NewRecorder()

		handler.CreateTransaction(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		var errResp rest.ErrorResponse
		err := json.Unmarshal(rr.Body.Bytes(), &errResp)
		assert.NoError(t, err)
		assert.Equal(t, "INVALID_DIRECTION", errResp.Code)
	})
}
