package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

type AccountStatus string

const (
	AccountStatusActive AccountStatus = "ACTIVE"
	AccountStatusFrozen AccountStatus = "FROZEN"
	AccountStatusClosed AccountStatus = "CLOSED"
)

type Account struct {
	ID        uuid.UUID     `json:"id"`
	Name      string        `json:"name"`
	Currency  string        `json:"currency"`
	Balance   int64         `json:"balance"` // In minor units (cents)
	Status    AccountStatus `json:"status"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
}

func NewAccount(name, currency string, initialBalance int64) (*Account, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errorsNew("account name cannot be empty")
	}

	currency = strings.ToUpper(strings.TrimSpace(currency))
	if len(currency) != 3 {
		return nil, ErrInvalidCurrency
	}

	if initialBalance < 0 {
		return nil, ErrNegativeAmount
	}

	now := time.Now().UTC()
	return &Account{
		ID:        uuid.New(),
		Name:      name,
		Currency:  currency,
		Balance:   initialBalance,
		Status:    AccountStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

func (a *Account) CanDebit(amount int64) bool {
	return a.Balance >= amount
}

func errorsNew(msg string) error {
	return &domainError{msg: msg}
}

type domainError struct {
	msg string
}

func (e *domainError) Error() string {
	return e.msg
}
