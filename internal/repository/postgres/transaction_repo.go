package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/ledger/transaction-engine/internal/domain"
)

type TransactionRepository struct {
	db *DB
}

func NewTransactionRepository(db *DB) *TransactionRepository {
	return &TransactionRepository{db: db}
}

type LedgerAuditResult struct {
	TotalDebits               int64    `json:"total_debits"`
	TotalCredits              int64    `json:"total_credits"`
	IsLedgerBalanced          bool     `json:"is_ledger_balanced"`
	UnbalancedTransactionIDs  []string `json:"unbalanced_transactions,omitempty"`
	MismatchedAccountBalances []string `json:"mismatched_accounts,omitempty"`
}

// ExecuteTransaction executes a double-entry transaction atomically using pessimistic locking
func (r *TransactionRepository) ExecuteTransaction(
	ctx context.Context,
	draft *domain.TransactionDraft,
	idempotencyKey string,
	rawPayload []byte,
	idempotencyExpiresAt time.Time,
) (*domain.Transaction, []byte, error) {
	if err := draft.Validate(); err != nil {
		return nil, nil, err
	}

	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to begin database transaction: %w", err)
	}
	defer tx.Rollback(ctx) // Safe no-op if tx.Commit succeeded

	// 1. Gather all unique account IDs
	accountIDMap := make(map[uuid.UUID]struct{})
	for _, e := range draft.Entries {
		accountIDMap[e.AccountID] = struct{}{}
	}

	uniqueAccountIDs := make([]uuid.UUID, 0, len(accountIDMap))
	for id := range accountIDMap {
		uniqueAccountIDs = append(uniqueAccountIDs, id)
	}

	// 2. Deadlock prevention: sort account IDs in deterministic ascending order
	sort.Slice(uniqueAccountIDs, func(i, j int) bool {
		return uniqueAccountIDs[i].String() < uniqueAccountIDs[j].String()
	})

	// 3. Acquire row locks deterministically with SELECT ... FOR UPDATE
	lockedAccounts := make(map[uuid.UUID]*domain.Account)
	queryLock := `
		SELECT id, name, currency, balance, status, created_at, updated_at
		FROM accounts
		WHERE id = ANY($1)
		ORDER BY id ASC
		FOR UPDATE
	`
	rows, err := tx.Query(ctx, queryLock, uniqueAccountIDs)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to acquire pessimistic locks on accounts: %w", err)
	}
	defer rows.Close()

	var firstCurrency string
	for rows.Next() {
		var acc domain.Account
		var status string
		if err := rows.Scan(
			&acc.ID,
			&acc.Name,
			&acc.Currency,
			&acc.Balance,
			&status,
			&acc.CreatedAt,
			&acc.UpdatedAt,
		); err != nil {
			return nil, nil, fmt.Errorf("failed to read locked account: %w", err)
		}
		acc.Status = domain.AccountStatus(status)
		lockedAccounts[acc.ID] = &acc

		if firstCurrency == "" {
			firstCurrency = acc.Currency
		} else if firstCurrency != acc.Currency {
			return nil, nil, domain.ErrCurrencyMismatch
		}
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("error reading locked account rows: %w", err)
	}

	// Ensure all accounts exist
	for _, id := range uniqueAccountIDs {
		acc, ok := lockedAccounts[id]
		if !ok {
			return nil, nil, fmt.Errorf("%w: %s", domain.ErrAccountNotFound, id.String())
		}
		if acc.Status != domain.AccountStatusActive {
			return nil, nil, fmt.Errorf("%w: %s", domain.ErrAccountInactive, id.String())
		}
	}

	// 4. Calculate net balance mutations and prepare entries
	txnID := uuid.New()
	now := time.Now().UTC()
	entries := make([]domain.Entry, 0, len(draft.Entries))

	// Track running balances for accounts within this transaction
	runningBalances := make(map[uuid.UUID]int64)
	for id, acc := range lockedAccounts {
		runningBalances[id] = acc.Balance
	}

	for _, eDraft := range draft.Entries {
		currentBal := runningBalances[eDraft.AccountID]
		var nextBal int64

		switch eDraft.Direction {
		case domain.EntryDirectionDebit:
			if currentBal < eDraft.Amount {
				return nil, nil, fmt.Errorf("%w (account %s: available %d, required %d)",
					domain.ErrInsufficientFunds, eDraft.AccountID, currentBal, eDraft.Amount)
			}
			nextBal = currentBal - eDraft.Amount
		case domain.EntryDirectionCredit:
			nextBal = currentBal + eDraft.Amount
		}

		runningBalances[eDraft.AccountID] = nextBal

		entry, err := domain.NewEntry(txnID, eDraft.AccountID, eDraft.Amount, eDraft.Direction, nextBal)
		if err != nil {
			return nil, nil, err
		}
		entries = append(entries, *entry)
	}

	// 5. Update account balances in database
	for id, finalBal := range runningBalances {
		_, err := tx.Exec(ctx, `
			UPDATE accounts
			SET balance = $1, updated_at = $2
			WHERE id = $3
		`, finalBal, now, id)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to update balance for account %s: %w", id, err)
		}
	}

	// 6. Insert parent transaction record
	_, err = tx.Exec(ctx, `
		INSERT INTO transactions (id, reference, description, status, created_at, posted_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, txnID, draft.Reference, draft.Description, string(domain.TransactionStatusPosted), now, now)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to insert transaction record: %w", err)
	}

	// 7. Insert immutable entries
	for _, entry := range entries {
		_, err := tx.Exec(ctx, `
			INSERT INTO entries (id, transaction_id, account_id, amount, direction, balance_after, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`, entry.ID, entry.TransactionID, entry.AccountID, entry.Amount, string(entry.Direction), entry.BalanceAfter, entry.CreatedAt)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to insert entry record: %w", err)
		}
	}

	txn := &domain.Transaction{
		ID:          txnID,
		Reference:   draft.Reference,
		Description: draft.Description,
		Status:      domain.TransactionStatusPosted,
		Entries:     entries,
		CreatedAt:   now,
		PostedAt:    now,
	}

	respBody, err := json.Marshal(txn)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal transaction response: %w", err)
	}

	// 8. If idempotency key provided, save completion inside the same atomic transaction
	if idempotencyKey != "" {
		reqHash := domain.HashPayload(rawPayload)
		_, err := tx.Exec(ctx, `
			INSERT INTO idempotency_keys (key, request_hash, status, response_code, response_body, created_at, updated_at, expires_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT (key) DO UPDATE
			SET status = EXCLUDED.status,
			    response_code = EXCLUDED.response_code,
			    response_body = EXCLUDED.response_body,
			    updated_at = EXCLUDED.updated_at
		`, idempotencyKey, reqHash, string(domain.IdempotencyStatusCompleted),
			http.StatusCreated, respBody, now, now, idempotencyExpiresAt)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to persist idempotency completion: %w", err)
		}
	}

	// 9. Commit transaction
	if err := tx.Commit(ctx); err != nil {
		return nil, nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return txn, respBody, nil
}

func (r *TransactionRepository) GetTransactionByID(ctx context.Context, id uuid.UUID) (*domain.Transaction, error) {
	queryTxn := `
		SELECT id, reference, description, status, created_at, posted_at
		FROM transactions
		WHERE id = $1
	`
	var txn domain.Transaction
	var status string
	err := r.db.Pool.QueryRow(ctx, queryTxn, id).Scan(
		&txn.ID,
		&txn.Reference,
		&txn.Description,
		&status,
		&txn.CreatedAt,
		&txn.PostedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("transaction not found")
		}
		return nil, fmt.Errorf("failed to query transaction: %w", err)
	}
	txn.Status = domain.TransactionStatus(status)

	// Fetch entries
	queryEntries := `
		SELECT id, transaction_id, account_id, amount, direction, balance_after, created_at
		FROM entries
		WHERE transaction_id = $1
		ORDER BY created_at ASC
	`
	rows, err := r.db.Pool.Query(ctx, queryEntries, id)
	if err != nil {
		return nil, fmt.Errorf("failed to query transaction entries: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var e domain.Entry
		var direction string
		if err := rows.Scan(
			&e.ID,
			&e.TransactionID,
			&e.AccountID,
			&e.Amount,
			&direction,
			&e.BalanceAfter,
			&e.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan entry: %w", err)
		}
		e.Direction = domain.EntryDirection(direction)
		txn.Entries = append(txn.Entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error reading entry rows: %w", err)
	}

	return &txn, nil
}

func (r *TransactionRepository) AuditLedger(ctx context.Context) (*LedgerAuditResult, error) {
	result := &LedgerAuditResult{}

	// 1. Calculate sum of all debits vs sum of all credits
	querySum := `
		SELECT
			COALESCE(SUM(CASE WHEN direction = 'DEBIT' THEN amount ELSE 0 END), 0) AS total_debits,
			COALESCE(SUM(CASE WHEN direction = 'CREDIT' THEN amount ELSE 0 END), 0) AS total_credits
		FROM entries;
	`
	err := r.db.Pool.QueryRow(ctx, querySum).Scan(&result.TotalDebits, &result.TotalCredits)
	if err != nil {
		return nil, fmt.Errorf("failed to audit entry totals: %w", err)
	}

	result.IsLedgerBalanced = (result.TotalDebits == result.TotalCredits)

	// 2. Identify any transaction where debits != credits
	queryUnbalanced := `
		SELECT transaction_id
		FROM entries
		GROUP BY transaction_id
		HAVING SUM(CASE WHEN direction = 'DEBIT' THEN amount ELSE 0 END) !=
		       SUM(CASE WHEN direction = 'CREDIT' THEN amount ELSE 0 END);
	`
	rows, err := r.db.Pool.Query(ctx, queryUnbalanced)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var txnID uuid.UUID
			if err := rows.Scan(&txnID); err == nil {
				result.UnbalancedTransactionIDs = append(result.UnbalancedTransactionIDs, txnID.String())
			}
		}
	}

	return result, nil
}
