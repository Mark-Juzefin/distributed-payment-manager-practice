package acquirer

import "context"

type AuthResult struct {
	Approved      bool
	DeclineReason string
}

type SettleResult struct {
	Success bool
	Reason  string
}

// Acquirer represents a bank/card network that processes authorization and settlement.
type Acquirer interface {
	Authorize(ctx context.Context, amount int64, currency, cardToken string) (AuthResult, error)
	Settle(ctx context.Context, txID string, amount int64) (SettleResult, error)
}
