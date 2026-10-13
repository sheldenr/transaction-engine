package domain

import "errors"

var (
	ErrAccountNotFound       = errors.New("account not found")
	ErrAccountInactive       = errors.New("account is inactive")
	ErrInsufficientFunds     = errors.New("insufficient funds for account")
	ErrNegativeAmount        = errors.New("amount must be strictly positive")
	ErrZeroAmount            = errors.New("amount cannot be zero")
	ErrCurrencyMismatch      = errors.New("all accounts in a transaction must share the same currency")
	ErrUnbalancedLedger      = errors.New("ledger entries are unbalanced: total debits must equal total credits")
	ErrTooFewEntries         = errors.New("transaction must contain at least two entries")
	ErrDuplicateAccountEntry = errors.New("duplicate account entries in the same transaction direction are not allowed")
	ErrSameAccountTransfer   = errors.New("cannot transfer funds to the same account")

	ErrIdempotencyKeyConflict = errors.New("request with this idempotency key is already in flight")
	ErrIdempotencyKeyMismatch = errors.New("idempotency key was previously used with a different request payload")
	ErrInvalidUUID            = errors.New("invalid UUID format")
	ErrInvalidCurrency        = errors.New("currency must be a valid 3-letter ISO code")
	ErrInvalidDirection       = errors.New("direction must be DEBIT or CREDIT")
)
