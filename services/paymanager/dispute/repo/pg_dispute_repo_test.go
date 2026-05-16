package repo

import (
	"TestTaskJustPay/services/paymanager/dispute"
	"TestTaskJustPay/services/paymanager/gateway"
	"context"
	"testing"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetDisputeByID(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	r := &repo{db: mock, readDB: mock, builder: squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)}
	ctx := context.Background()

	t.Run("should return dispute with basic query", func(t *testing.T) {
		disputeID := "dispute-1"
		expectedTime := time.Now()

		rows := mock.NewRows([]string{"id", "order_id", "submitting_id", "status", "reason", "amount", "currency", "opened_at", "evidence_due_at", "submitted_at", "closed_at"}).
			AddRow(disputeID, "order-1", nil, "open", "fraud", 100.50, "USD", expectedTime, nil, nil, nil)

		mock.ExpectQuery(`SELECT id, order_id, submitting_id, status, reason, amount, currency, opened_at, evidence_due_at, submitted_at, closed_at FROM disputes WHERE id = \$1`).
			WithArgs(disputeID).
			WillReturnRows(rows)

		result, err := r.GetDisputeByID(ctx, disputeID)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, disputeID, result.ID)
		assert.Equal(t, "order-1", result.OrderID)
		assert.Equal(t, dispute.DisputeOpen, result.Status)
		assert.Equal(t, "fraud", result.Reason)
		assert.Equal(t, 100.50, result.Amount)
		assert.Equal(t, "USD", result.Currency)
		assert.Nil(t, result.EvidenceDueAt)
		assert.Nil(t, result.SubmittedAt)
		assert.Nil(t, result.ClosedAt)
	})

	t.Run("should return nil when dispute not found", func(t *testing.T) {
		disputeID := "nonexistent"

		mock.ExpectQuery(`SELECT id, order_id, submitting_id, status, reason, amount, currency, opened_at, evidence_due_at, submitted_at, closed_at FROM disputes WHERE id = \$1`).
			WithArgs(disputeID).
			WillReturnRows(pgxmock.NewRows([]string{"id", "order_id", "submitting_id", "status", "reason", "amount", "currency", "opened_at", "evidence_due_at", "submitted_at", "closed_at"}))

		result, err := r.GetDisputeByID(ctx, disputeID)

		require.NoError(t, err)
		assert.Nil(t, result)
	})

	t.Run("should handle database error", func(t *testing.T) {
		disputeID := "dispute-1"

		mock.ExpectQuery(`SELECT id, order_id, submitting_id, status, reason, amount, currency, opened_at, evidence_due_at, submitted_at, closed_at FROM disputes WHERE id = \$1`).
			WithArgs(disputeID).
			WillReturnError(assert.AnError)

		result, err := r.GetDisputeByID(ctx, disputeID)

		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "query dispute by id")
	})
}

func TestGetDisputeByOrderID(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	r := &repo{db: mock, readDB: mock, builder: squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)}
	ctx := context.Background()

	t.Run("should return dispute by order ID", func(t *testing.T) {
		orderID := "order-1"
		expectedTime := time.Now()
		evidenceTime := expectedTime.Add(7 * 24 * time.Hour)

		rows := mock.NewRows([]string{"id", "order_id", "submitting_id", "status", "reason", "amount", "currency", "opened_at", "evidence_due_at", "submitted_at", "closed_at"}).
			AddRow("dispute-1", orderID, nil, "open", "fraud", 100.50, "USD", expectedTime, evidenceTime, nil, nil)

		mock.ExpectQuery(`SELECT id, order_id, submitting_id, status, reason, amount, currency, opened_at, evidence_due_at, submitted_at, closed_at FROM disputes WHERE order_id = \$1`).
			WithArgs(orderID).
			WillReturnRows(rows)

		result, err := r.GetDisputeByOrderID(ctx, orderID)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "dispute-1", result.ID)
		assert.Equal(t, orderID, result.OrderID)
		assert.NotNil(t, result.EvidenceDueAt)
		assert.Equal(t, evidenceTime, *result.EvidenceDueAt)
	})
}

func TestCreateDispute(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	r := &repo{db: mock, readDB: mock, builder: squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)}
	ctx := context.Background()

	t.Run("should create dispute successfully", func(t *testing.T) {
		openedAt := time.Now()
		evidenceDueAt := openedAt.Add(7 * 24 * time.Hour)

		newDispute := dispute.NewDispute{
			Status: dispute.DisputeOpen,
			DisputeInfo: dispute.DisputeInfo{
				OrderID: "order-1",
				Reason:  "fraud",
				Money: dispute.Money{
					Amount:   100.50,
					Currency: "USD",
				},
				OpenedAt:      openedAt,
				EvidenceDueAt: &evidenceDueAt,
			},
		}

		mock.ExpectExec(`INSERT INTO disputes \(id,order_id,submitting_id,status,reason,amount,currency,opened_at,evidence_due_at,submitted_at,closed_at\) VALUES \(\$1,\$2,\$3,\$4,\$5,\$6,\$7,\$8,\$9,\$10,\$11\)`).
			WithArgs(pgxmock.AnyArg(), "order-1", (*string)(nil), dispute.DisputeOpen, "fraud", 100.50, "USD", openedAt, &evidenceDueAt, (*time.Time)(nil), (*time.Time)(nil)).
			WillReturnResult(pgxmock.NewResult("INSERT", 1))

		result, err := r.CreateDispute(ctx, newDispute)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.NotEmpty(t, result.ID)
		assert.Equal(t, dispute.DisputeOpen, result.Status)
		assert.Equal(t, "order-1", result.OrderID)
	})
}

func TestUpsertEvidence(t *testing.T) {
	t.Run("should upsert evidence successfully", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		r := &repo{db: mock, readDB: mock, builder: squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)}
		ctx := context.Background()

		disputeID := "dispute-1"
		upsert := dispute.EvidenceUpsert{
			Evidence: gateway.Evidence{
				Fields: map[string]string{
					"transaction_receipt":    "receipt_123",
					"customer_communication": "email_456",
				},
				Files: []gateway.EvidenceFile{
					{FileID: "file-1", Name: "receipt.pdf", ContentType: "application/pdf", Size: 1024},
				},
			},
		}

		expectedFieldsJSON := []byte(`{"customer_communication":"email_456","transaction_receipt":"receipt_123"}`)
		expectedFilesJSON := []byte(`[{"file_id":"file-1","name":"receipt.pdf","content_type":"application/pdf","size":1024}]`)

		mock.ExpectExec(`INSERT INTO evidence \(dispute_id,fields,files,updated_at\) VALUES \(\$1,\$2,\$3,\$4\) ON CONFLICT \(dispute_id\) DO UPDATE SET fields = EXCLUDED\.fields, files = EXCLUDED\.files, updated_at = EXCLUDED\.updated_at`).
			WithArgs(disputeID, expectedFieldsJSON, expectedFilesJSON, pgxmock.AnyArg()).
			WillReturnResult(pgxmock.NewResult("INSERT", 1))

		result, err := r.UpsertEvidence(ctx, disputeID, upsert)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, disputeID, result.DisputeID)
	})
}

func TestGetEvidence(t *testing.T) {
	t.Run("should return evidence for dispute", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		r := &repo{db: mock, readDB: mock, builder: squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)}
		ctx := context.Background()

		disputeID := "dispute-1"
		updatedAt := time.Now()
		fieldsJSON := `{"transaction_receipt":"receipt_123"}`
		filesJSON := `[{"file_id":"file-1","name":"receipt.pdf","content_type":"application/pdf","size":1024}]`

		rows := mock.NewRows([]string{"dispute_id", "fields", "files", "updated_at"}).
			AddRow(disputeID, fieldsJSON, filesJSON, updatedAt)

		mock.ExpectQuery(`SELECT dispute_id, fields, files, updated_at FROM evidence WHERE dispute_id = \$1`).
			WithArgs(disputeID).
			WillReturnRows(rows)

		result, err := r.GetEvidence(ctx, disputeID)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, disputeID, result.DisputeID)
		assert.Equal(t, "receipt_123", result.Fields["transaction_receipt"])
		assert.Len(t, result.Files, 1)
		assert.Equal(t, "file-1", result.Files[0].FileID)
		assert.Equal(t, updatedAt, result.UpdatedAt)
	})

	t.Run("should return nil when evidence not found", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		r := &repo{db: mock, readDB: mock, builder: squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)}
		ctx := context.Background()

		disputeID := "nonexistent"

		mock.ExpectQuery(`SELECT dispute_id, fields, files, updated_at FROM evidence WHERE dispute_id = \$1`).
			WithArgs(disputeID).
			WillReturnRows(pgxmock.NewRows([]string{"dispute_id", "fields", "files", "updated_at"}))

		result, err := r.GetEvidence(ctx, disputeID)

		require.NoError(t, err)
		assert.Nil(t, result)
	})
}
