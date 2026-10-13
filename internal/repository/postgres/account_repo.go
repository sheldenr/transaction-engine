package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/ledger/transaction-engine/internal/domain"
)

type AccountRepository struct {
	db *DB
}

func NewAccountRepository(db *DB) *AccountRepository {
	return &AccountRepository{db: db}
}

func (r *AccountRepository) CreateAccount(ctx context.Context, acc *domain.Account) error {
	query := `
		INSERT INTO accounts (id, name, currency, balance, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := r.db.Pool.Exec(ctx, query,
		acc.ID,
		acc.Name,
		acc.Currency,
		acc.Balance,
		string(acc.Status),
		acc.CreatedAt,
		acc.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create account: %w", err)
	}
	return nil
}

func (r *AccountRepository) GetAccountByID(ctx context.Context, id uuid.UUID) (*domain.Account, error) {
	query := `
		SELECT id, name, currency, balance, status, created_at, updated_at
		FROM accounts
		WHERE id = $1
	`
	var acc domain.Account
	var status string
	err := r.db.Pool.QueryRow(ctx, query, id).Scan(
		&acc.ID,
		&acc.Name,
		&acc.Currency,
		&acc.Balance,
		&status,
		&acc.CreatedAt,
		&acc.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrAccountNotFound
		}
		return nil, fmt.Errorf("failed to query account: %w", err)
	}
	acc.Status = domain.AccountStatus(status)
	return &acc, nil
}

func (r *AccountRepository) ListAccounts(ctx context.Context, limit, offset int) ([]domain.Account, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	query := `
		SELECT id, name, currency, balance, status, created_at, updated_at
		FROM accounts
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`
	rows, err := r.db.Pool.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list accounts: %w", err)
	}
	defer rows.Close()

	var accounts []domain.Account
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
			return nil, fmt.Errorf("failed to scan account: %w", err)
		}
		acc.Status = domain.AccountStatus(status)
		accounts = append(accounts, acc)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row error listing accounts: %w", err)
	}

	return accounts, nil
}

func (r *AccountRepository) GetEntriesByAccountID(ctx context.Context, accountID uuid.UUID, limit, offset int) ([]domain.Entry, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	query := `
		SELECT id, transaction_id, account_id, amount, direction, balance_after, created_at
		FROM entries
		WHERE account_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.db.Pool.Query(ctx, query, accountID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query entries for account: %w", err)
	}
	defer rows.Close()

	var entries []domain.Entry
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
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row error reading entries: %w", err)
	}

	return entries, nil
}
