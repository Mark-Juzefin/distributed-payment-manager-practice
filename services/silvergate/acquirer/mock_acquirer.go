package acquirer

import (
	"context"
	"math/rand/v2"
	"time"
)

var declineReasons = []string{
	"insufficient_funds",
	"card_expired",
	"do_not_honor",
	"suspected_fraud",
}

// MockAcquirer simulates a bank with configurable approve/settle rates.
type MockAcquirer struct {
	AuthApproveRate   float64       // 0.0–1.0, probability of auth approval
	SettleSuccessRate float64       // 0.0–1.0, probability of settle success
	SettleDelay       time.Duration // simulated settlement processing time
}

func NewMockAcquirer(authRate, settleRate float64, settleDelay time.Duration) *MockAcquirer {
	return &MockAcquirer{
		AuthApproveRate:   authRate,
		SettleSuccessRate: settleRate,
		SettleDelay:       settleDelay,
	}
}

func (m *MockAcquirer) Authorize(_ context.Context, _ int64, _, _ string) (AuthResult, error) {
	if rand.Float64() < m.AuthApproveRate {
		return AuthResult{Approved: true}, nil
	}
	reason := declineReasons[rand.IntN(len(declineReasons))]
	return AuthResult{Approved: false, DeclineReason: reason}, nil
}

func (m *MockAcquirer) Void(_ context.Context, _ string) (VoidResult, error) {
	return VoidResult{Success: true}, nil
}

func (m *MockAcquirer) Settle(_ context.Context, _ string, _ int64) (SettleResult, error) {
	if m.SettleDelay > 0 {
		time.Sleep(m.SettleDelay)
	}
	if rand.Float64() < m.SettleSuccessRate {
		return SettleResult{Success: true}, nil
	}
	return SettleResult{Success: false, Reason: "settlement_rejected"}, nil
}
