package store

import (
	"context"
	"time"
)

type CardStatus string

type Card struct {
	ID               string
	PAN              string
	ExpiryDate       string
	Status           CardStatus
	Scheme           string
	CurrencyCode     string
	CreditLimit      int64
	AvailableBalance int64
	PinHash          *string
	CvvHash          *string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type TransactionStatus string

type AcquirerTransaction struct {
	ID           string
	STAN         string
	RRN          *string
	MTI          string
	PANMasked    string
	Amount       int64
	CurrencyCode string
	TerminalID   *string
	MerchantID   *string
	Status       TransactionStatus
	ResponseCode *string
	RequestHex   *string
	ResponseHex  *string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type IssuerTransaction struct {
	ID               string
	STAN             string
	RRN              *string
	MTI              string
	PANMasked        string
	Amount           int64
	CurrencyCode     string
	Status           TransactionStatus
	ResponseCode     *string
	AuthCode         *string
	DeclineReason    *string
	BalanceBefore    *int64
	BalanceAfter     *int64
	ProcessingTimeMs *float64
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// AuthorizeDebitResult holds the result of an atomic card lookup + debit operation.
type AuthorizeDebitResult struct {
	Card          *Card // Always populated if card found (nil if not found)
	BalanceBefore int64 // Set only when Debited == true
	BalanceAfter  int64 // Set only when Debited == true
	Debited       bool  // True if balance was sufficient and deducted
}

type CardStore interface {
	CreateCard(ctx context.Context, c *Card) (*Card, error)
	GetCardByID(ctx context.Context, id string) (*Card, error)
	GetCardByPAN(ctx context.Context, pan string) (*Card, error)
	CreditBalance(ctx context.Context, id string, amount int64) error
	ListCards(ctx context.Context, limit int, offset int) ([]Card, error)
	UpdateCard(ctx context.Context, c *Card) (*Card, error)
	SoftDeleteCard(ctx context.Context, id string) error
	UpdateCardBalance(ctx context.Context, id string, newAvailableBalance int64) error
	DebitIfSufficient(ctx context.Context, id string, amount int64) (before int64, after int64, ok bool, err error)
	AuthorizeAndDebit(ctx context.Context, pan string, amount int64) (*AuthorizeDebitResult, error)
}

type AcquirerTransactionStore interface {
	CreateAcquirerTransaction(ctx context.Context, t *AcquirerTransaction) (*AcquirerTransaction, error)
	UpdateAcquirerTransactionStatus(ctx context.Context, id string, status TransactionStatus, responseCode *string, responseHex *string) error
	ListAcquirerTransactions(ctx context.Context, limit int, offset int) ([]AcquirerTransaction, error)
}

type IssuerTransactionStore interface {
	CreateIssuerTransaction(ctx context.Context, t *IssuerTransaction) (*IssuerTransaction, error)
	UpdateIssuerTransaction(ctx context.Context, id string, status TransactionStatus, responseCode *string, authCode *string, declineReason *string, balanceBefore *int64, balanceAfter *int64, processingTimeMs *float64) error
	ListIssuerTransactions(ctx context.Context, limit int, offset int) ([]IssuerTransaction, error)
}

type Store interface {
	Cards() CardStore
	AcquirerTransactions() AcquirerTransactionStore
	IssuerTransactions() IssuerTransactionStore
	Close() error
}
