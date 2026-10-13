package dto

import (
	"github.com/google/uuid"
)

type CreateAccountRequest struct {
	Name           string `json:"name"`
	Currency       string `json:"currency"`
	InitialBalance int64  `json:"initial_balance"`
}

type CreateEntryRequest struct {
	AccountID uuid.UUID `json:"account_id"`
	Amount    int64     `json:"amount"`
	Direction string    `json:"direction"`
}

type CreateTransactionRequest struct {
	Reference   string               `json:"reference"`
	Description string               `json:"description"`
	Entries     []CreateEntryRequest `json:"entries"`
}

type CreateTransferRequest struct {
	SourceAccountID      uuid.UUID `json:"source_account_id"`
	DestinationAccountID uuid.UUID `json:"destination_account_id"`
	Amount               int64     `json:"amount"`
	Reference            string    `json:"reference"`
	Description          string    `json:"description"`
}
