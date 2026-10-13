package domain_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/ledger/transaction-engine/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAccount(t *testing.T) {
	t.Run("Valid account creation", func(t *testing.T) {
		acc, err := domain.NewAccount("Alice", "USD", 1000)
		require.NoError(t, err)
		assert.Equal(t, "Alice", acc.Name)
		assert.Equal(t, "USD", acc.Currency)
		assert.Equal(t, int64(1000), acc.Balance)
		assert.Equal(t, domain.AccountStatusActive, acc.Status)
		assert.NotEqual(t, uuid.Nil, acc.ID)
	})

	t.Run("Negative initial balance rejected", func(t *testing.T) {
		_, err := domain.NewAccount("Bob", "USD", -50)
		require.ErrorIs(t, err, domain.ErrNegativeAmount)
	})

	t.Run("Invalid currency rejected", func(t *testing.T) {
		_, err := domain.NewAccount("Bob", "US", 100)
		require.ErrorIs(t, err, domain.ErrInvalidCurrency)

		_, err = domain.NewAccount("Bob", "USDD", 100)
		require.ErrorIs(t, err, domain.ErrInvalidCurrency)
	})

	t.Run("Empty account name rejected", func(t *testing.T) {
		_, err := domain.NewAccount("   ", "USD", 100)
		require.Error(t, err)
	})
}

func TestTransactionDraft_Validate(t *testing.T) {
	acc1 := uuid.New()
	acc2 := uuid.New()
	acc3 := uuid.New()

	t.Run("Balanced double-entry succeeds", func(t *testing.T) {
		draft := domain.TransactionDraft{
			Reference:   "REF-001",
			Description: "Test transfer",
			Entries: []domain.EntryDraft{
				{AccountID: acc1, Amount: 500, Direction: domain.EntryDirectionDebit},
				{AccountID: acc2, Amount: 500, Direction: domain.EntryDirectionCredit},
			},
		}
		require.NoError(t, draft.Validate())
	})

	t.Run("Balanced multi-split transaction succeeds", func(t *testing.T) {
		draft := domain.TransactionDraft{
			Reference:   "REF-SPLIT",
			Description: "One debit, two credits",
			Entries: []domain.EntryDraft{
				{AccountID: acc1, Amount: 1000, Direction: domain.EntryDirectionDebit},
				{AccountID: acc2, Amount: 600, Direction: domain.EntryDirectionCredit},
				{AccountID: acc3, Amount: 400, Direction: domain.EntryDirectionCredit},
			},
		}
		require.NoError(t, draft.Validate())
	})

	t.Run("Unbalanced transaction rejected", func(t *testing.T) {
		draft := domain.TransactionDraft{
			Reference:   "REF-FAIL",
			Description: "Debits do not match credits",
			Entries: []domain.EntryDraft{
				{AccountID: acc1, Amount: 500, Direction: domain.EntryDirectionDebit},
				{AccountID: acc2, Amount: 499, Direction: domain.EntryDirectionCredit},
			},
		}
		require.ErrorIs(t, draft.Validate(), domain.ErrUnbalancedLedger)
	})

	t.Run("Too few entries rejected", func(t *testing.T) {
		draft := domain.TransactionDraft{
			Reference:   "REF-ONE",
			Description: "Single entry only",
			Entries: []domain.EntryDraft{
				{AccountID: acc1, Amount: 500, Direction: domain.EntryDirectionDebit},
			},
		}
		require.ErrorIs(t, draft.Validate(), domain.ErrTooFewEntries)
	})

	t.Run("Zero or negative amount rejected", func(t *testing.T) {
		draft := domain.TransactionDraft{
			Reference:   "REF-ZERO",
			Description: "Zero amount",
			Entries: []domain.EntryDraft{
				{AccountID: acc1, Amount: 0, Direction: domain.EntryDirectionDebit},
				{AccountID: acc2, Amount: 0, Direction: domain.EntryDirectionCredit},
			},
		}
		require.ErrorIs(t, draft.Validate(), domain.ErrNegativeAmount)
	})
}

func TestIdempotencyPayloadHash(t *testing.T) {
	payload1 := []byte(`{"reference":"123","amount":100}`)
	payload2 := []byte(`{"reference":"123","amount":100}`)
	payload3 := []byte(`{"reference":"123","amount":200}`)

	hash1 := domain.HashPayload(payload1)
	hash2 := domain.HashPayload(payload2)
	hash3 := domain.HashPayload(payload3)

	assert.Equal(t, hash1, hash2)
	assert.NotEqual(t, hash1, hash3)
	assert.Len(t, hash1, 64) // SHA-256 hex string
}
