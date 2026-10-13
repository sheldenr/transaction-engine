package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

type EntryDirection string

const (
	EntryDirectionDebit  EntryDirection = "DEBIT"
	EntryDirectionCredit EntryDirection = "CREDIT"
)

func ParseEntryDirection(s string) (EntryDirection, error) {
	upper := strings.ToUpper(strings.TrimSpace(s))
	switch EntryDirection(upper) {
	case EntryDirectionDebit:
		return EntryDirectionDebit, nil
	case EntryDirectionCredit:
		return EntryDirectionCredit, nil
	default:
		return "", ErrInvalidDirection
	}
}

type Entry struct {
	ID            uuid.UUID      `json:"id"`
	TransactionID uuid.UUID      `json:"transaction_id"`
	AccountID     uuid.UUID      `json:"account_id"`
	Amount        int64          `json:"amount"` // Strictly positive minor units
	Direction     EntryDirection `json:"direction"`
	BalanceAfter  int64          `json:"balance_after"`
	CreatedAt     time.Time      `json:"created_at"`
}

func NewEntry(txnID, accountID uuid.UUID, amount int64, direction EntryDirection, balanceAfter int64) (*Entry, error) {
	if amount <= 0 {
		return nil, ErrNegativeAmount
	}
	if direction != EntryDirectionDebit && direction != EntryDirectionCredit {
		return nil, ErrInvalidDirection
	}

	return &Entry{
		ID:            uuid.New(),
		TransactionID: txnID,
		AccountID:     accountID,
		Amount:        amount,
		Direction:     direction,
		BalanceAfter:  balanceAfter,
		CreatedAt:     time.Now().UTC(),
	}, nil
}
