package domain

import (
	"time"

	"github.com/google/uuid"
)

type TransactionStatus string

const (
	TransactionStatusPosted   TransactionStatus = "POSTED"
	TransactionStatusRejected TransactionStatus = "REJECTED"
)

type Transaction struct {
	ID          uuid.UUID         `json:"id"`
	Reference   string            `json:"reference"`
	Description string            `json:"description"`
	Status      TransactionStatus `json:"status"`
	Entries     []Entry           `json:"entries"`
	CreatedAt   time.Time         `json:"created_at"`
	PostedAt    time.Time         `json:"posted_at"`
}

// TransactionDraft represents an intent to post a transaction before persisting.
type TransactionDraft struct {
	Reference   string
	Description string
	Entries     []EntryDraft
}

type EntryDraft struct {
	AccountID uuid.UUID
	Amount    int64
	Direction EntryDirection
}

// Validate ensures double-entry invariant: sum(Debits) == sum(Credits) > 0
func (d *TransactionDraft) Validate() error {
	if len(d.Entries) < 2 {
		return ErrTooFewEntries
	}

	var totalDebits int64
	var totalCredits int64

	for _, e := range d.Entries {
		if e.AccountID == uuid.Nil {
			return ErrInvalidUUID
		}
		if e.Amount <= 0 {
			return ErrNegativeAmount
		}

		switch e.Direction {
		case EntryDirectionDebit:
			totalDebits += e.Amount
		case EntryDirectionCredit:
			totalCredits += e.Amount
		default:
			return ErrInvalidDirection
		}
	}

	if totalDebits == 0 || totalCredits == 0 {
		return ErrZeroAmount
	}

	if totalDebits != totalCredits {
		return ErrUnbalancedLedger
	}

	return nil
}
