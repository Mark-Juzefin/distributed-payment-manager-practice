package payment

import (
	"context"

	"TestTaskJustPay/services/paymanager/internal/gateway"
)

// PaymentRepo is the persistence contract for payments.
type PaymentRepo interface {
	CreatePayment(ctx context.Context, payment Payment) error
	GetPaymentByID(ctx context.Context, id string) (*Payment, error)
	GetPaymentByProviderTxID(ctx context.Context, txID string) (*Payment, error)
	UpdatePaymentStatus(ctx context.Context, id string, status Status, declineReason string) error
	UpdatePaymentRefund(ctx context.Context, id string, status Status, refundedAmount int64) error
}

// Provider is the minimal interface this domain requires from the payment gateway.
type Provider interface {
	AuthorizePayment(ctx context.Context, req gateway.AuthRequest) (gateway.AuthResult, error)
	CapturePayment(ctx context.Context, req gateway.CaptureRequest) (gateway.CaptureResult, error)
	VoidPayment(ctx context.Context, req gateway.VoidRequest) (gateway.VoidResult, error)
	RefundPayment(ctx context.Context, req gateway.RefundRequest) (gateway.RefundResult, error)
}
