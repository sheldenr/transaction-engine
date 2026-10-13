package service

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/ledger/transaction-engine/internal/domain"
	"github.com/ledger/transaction-engine/internal/repository/postgres"
)

type LedgerService struct {
	accRepo            *postgres.AccountRepository
	txnRepo            *postgres.TransactionRepository
	idempotencyService *IdempotencyService
	idempotencyTTL     time.Duration
}

func NewLedgerService(
	accRepo *postgres.AccountRepository,
	txnRepo *postgres.TransactionRepository,
	idempotencyService *IdempotencyService,
	idempotencyTTL time.Duration,
) *LedgerService {
	return &LedgerService{
		accRepo:            accRepo,
		txnRepo:            txnRepo,
		idempotencyService: idempotencyService,
		idempotencyTTL:     idempotencyTTL,
	}
}

func (s *LedgerService) CreateAccount(ctx context.Context, name, currency string, initialBalance int64) (*domain.Account, error) {
	acc, err := domain.NewAccount(name, currency, initialBalance)
	if err != nil {
		return nil, err
	}

	if err := s.accRepo.CreateAccount(ctx, acc); err != nil {
		return nil, err
	}

	return acc, nil
}

func (s *LedgerService) GetAccount(ctx context.Context, id uuid.UUID) (*domain.Account, error) {
	return s.accRepo.GetAccountByID(ctx, id)
}

func (s *LedgerService) ListAccounts(ctx context.Context, limit, offset int) ([]domain.Account, error) {
	return s.accRepo.ListAccounts(ctx, limit, offset)
}

func (s *LedgerService) GetAccountHistory(ctx context.Context, id uuid.UUID, limit, offset int) ([]domain.Entry, error) {
	// Verify account exists
	if _, err := s.accRepo.GetAccountByID(ctx, id); err != nil {
		return nil, err
	}
	return s.accRepo.GetEntriesByAccountID(ctx, id, limit, offset)
}

// TransactionExecutionOutput holds the result of transaction execution or idempotency replay
type TransactionExecutionOutput struct {
	Transaction *domain.Transaction
	IsReplay    bool
	StatusCode  int
	CachedBody  []byte
}

func (s *LedgerService) ExecuteTransaction(
	ctx context.Context,
	draft *domain.TransactionDraft,
	idempotencyKey string,
	rawPayload []byte,
) (*TransactionExecutionOutput, error) {
	// 1. Validate domain invariants upfront
	if err := draft.Validate(); err != nil {
		return nil, err
	}

	// 2. Check Idempotency and acquire lock
	checkResult, err := s.idempotencyService.PrepareExecution(ctx, idempotencyKey, rawPayload)
	if err != nil {
		return nil, err
	}
	defer checkResult.Unlock()

	// If previously executed, return the replayed response directly
	if checkResult.IsReplay {
		var txn domain.Transaction
		if err := json.Unmarshal(checkResult.ResponseBody, &txn); err == nil {
			return &TransactionExecutionOutput{
				Transaction: &txn,
				IsReplay:    true,
				StatusCode:  checkResult.StatusCode,
				CachedBody:  checkResult.ResponseBody,
			}, nil
		}
		return &TransactionExecutionOutput{
			IsReplay:   true,
			StatusCode: checkResult.StatusCode,
			CachedBody: checkResult.ResponseBody,
		}, nil
	}

	// 3. Execute atomic transaction in PostgreSQL with row-level locks
	expiresAt := time.Now().UTC().Add(s.idempotencyTTL)

	txn, respBody, err := s.txnRepo.ExecuteTransaction(ctx, draft, idempotencyKey, rawPayload, expiresAt)
	if err != nil {
		// Clean up reservation if execution failed before commit
		s.idempotencyService.FailExecution(ctx, idempotencyKey)
		return nil, err
	}

	// Populate Redis fast cache with response
	_ = s.idempotencyService.CompleteExecution(ctx, idempotencyKey, rawPayload, http.StatusCreated, respBody)

	return &TransactionExecutionOutput{
		Transaction: txn,
		IsReplay:    false,
		StatusCode:  http.StatusCreated,
		CachedBody:  respBody,
	}, nil
}

// TransferFunds creates a double-entry debit from source and credit to destination
func (s *LedgerService) TransferFunds(
	ctx context.Context,
	sourceID, destinationID uuid.UUID,
	amount int64,
	reference, description string,
	idempotencyKey string,
	rawPayload []byte,
) (*TransactionExecutionOutput, error) {
	if sourceID == destinationID {
		return nil, domain.ErrSameAccountTransfer
	}
	if amount <= 0 {
		return nil, domain.ErrNegativeAmount
	}

	draft := &domain.TransactionDraft{
		Reference:   reference,
		Description: description,
		Entries: []domain.EntryDraft{
			{
				AccountID: sourceID,
				Amount:    amount,
				Direction: domain.EntryDirectionDebit,
			},
			{
				AccountID: destinationID,
				Amount:    amount,
				Direction: domain.EntryDirectionCredit,
			},
		},
	}

	return s.ExecuteTransaction(ctx, draft, idempotencyKey, rawPayload)
}

func (s *LedgerService) GetTransaction(ctx context.Context, id uuid.UUID) (*domain.Transaction, error) {
	return s.txnRepo.GetTransactionByID(ctx, id)
}

func (s *LedgerService) AuditLedger(ctx context.Context) (*postgres.LedgerAuditResult, error) {
	return s.txnRepo.AuditLedger(ctx)
}
