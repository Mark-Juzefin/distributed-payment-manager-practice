package payment

import "context"

type PaymentRepo interface {
	CreatePayment(ctx context.Context, payment Payment) error
	GetPaymentByID(ctx context.Context, id string) (*Payment, error)
	GetPaymentByProviderTxID(ctx context.Context, txID string) (*Payment, error)
	UpdatePaymentStatus(ctx context.Context, id string, status Status, declineReason string) error
}
