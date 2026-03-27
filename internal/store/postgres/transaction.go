package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"iso-parser-service/internal/store"
)

type AcquirerTransactionRepository struct {
	pool *pgxpool.Pool
}

type IssuerTransactionRepository struct {
	pool *pgxpool.Pool
}

func (r *AcquirerTransactionRepository) CreateAcquirerTransaction(ctx context.Context, t *store.AcquirerTransaction) (*store.AcquirerTransaction, error) {
	if t == nil {
		return nil, fmt.Errorf("transaction is nil")
	}

	row := r.pool.QueryRow(ctx, `
		INSERT INTO acquirer_transactions (
			stan, rrn, mti, pan_masked, amount, currency_code,
			terminal_id, merchant_id, status, response_code,
			request_hex, response_hex
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		RETURNING id, created_at, updated_at
	`, t.STAN, t.RRN, t.MTI, t.PANMasked, t.Amount, t.CurrencyCode, t.TerminalID, t.MerchantID, string(t.Status), t.ResponseCode, t.RequestHex, t.ResponseHex)

	if err := row.Scan(&t.ID, &t.CreatedAt, &t.UpdatedAt); err != nil {
		return nil, fmt.Errorf("insert acquirer transaction: %w", err)
	}

	return t, nil
}

func (r *AcquirerTransactionRepository) UpdateAcquirerTransactionStatus(ctx context.Context, id string, status store.TransactionStatus, responseCode *string, responseHex *string) error {
	cmd, err := r.pool.Exec(ctx, `
		UPDATE acquirer_transactions
		SET status = $2, response_code = $3, response_hex = $4, updated_at = NOW()
		WHERE id = $1
	`, id, string(status), responseCode, responseHex)
	if err != nil {
		return fmt.Errorf("update acquirer transaction: %w", err)
	}
	if cmd.RowsAffected() != 1 {
		return fmt.Errorf("update acquirer transaction: expected 1 row affected, got %d", cmd.RowsAffected())
	}
	return nil
}

func (r *AcquirerTransactionRepository) ListAcquirerTransactions(ctx context.Context, limit int, offset int) ([]store.AcquirerTransaction, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, stan, rrn, mti, pan_masked, amount, currency_code,
		       terminal_id, merchant_id, status, response_code,
		       request_hex, response_hex, created_at, updated_at
		FROM acquirer_transactions
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list acquirer transactions: %w", err)
	}
	defer rows.Close()

	var txs []store.AcquirerTransaction
	for rows.Next() {
		var t store.AcquirerTransaction
		if err := rows.Scan(
			&t.ID, &t.STAN, &t.RRN, &t.MTI, &t.PANMasked, &t.Amount, &t.CurrencyCode,
			&t.TerminalID, &t.MerchantID, &t.Status, &t.ResponseCode,
			&t.RequestHex, &t.ResponseHex, &t.CreatedAt, &t.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan acquirer transaction: %w", err)
		}
		txs = append(txs, t)
	}
	return txs, rows.Err()
}

func (r *IssuerTransactionRepository) CreateIssuerTransaction(ctx context.Context, t *store.IssuerTransaction) (*store.IssuerTransaction, error) {
	if t == nil {
		return nil, fmt.Errorf("transaction is nil")
	}

	row := r.pool.QueryRow(ctx, `
		INSERT INTO issuer_transactions (
			stan, rrn, mti, pan_masked, amount, currency_code,
			status, response_code, auth_code, decline_reason,
			balance_before, balance_after, processing_time_ms
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		RETURNING id, created_at, updated_at
	`, t.STAN, t.RRN, t.MTI, t.PANMasked, t.Amount, t.CurrencyCode, string(t.Status), t.ResponseCode, t.AuthCode, t.DeclineReason, t.BalanceBefore, t.BalanceAfter, t.ProcessingTimeMs)

	if err := row.Scan(&t.ID, &t.CreatedAt, &t.UpdatedAt); err != nil {
		return nil, fmt.Errorf("insert issuer transaction: %w", err)
	}

	return t, nil
}

func (r *IssuerTransactionRepository) ListIssuerTransactions(ctx context.Context, limit int, offset int) ([]store.IssuerTransaction, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id, stan, rrn, mti, pan_masked, amount, currency_code,
		       status, response_code, auth_code, decline_reason,
		       balance_before, balance_after, processing_time_ms,
		       created_at, updated_at
		FROM issuer_transactions
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list issuer transactions: %w", err)
	}
	defer rows.Close()

	var txs []store.IssuerTransaction
	for rows.Next() {
		var t store.IssuerTransaction
		var status string
		if err := rows.Scan(
			&t.ID, &t.STAN, &t.RRN, &t.MTI, &t.PANMasked, &t.Amount, &t.CurrencyCode,
			&status, &t.ResponseCode, &t.AuthCode, &t.DeclineReason,
			&t.BalanceBefore, &t.BalanceAfter, &t.ProcessingTimeMs,
			&t.CreatedAt, &t.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan issuer transaction: %w", err)
		}
		t.Status = store.TransactionStatus(status)
		txs = append(txs, t)
	}
	return txs, rows.Err()
}

func (r *IssuerTransactionRepository) UpdateIssuerTransaction(ctx context.Context, id string, status store.TransactionStatus, responseCode *string, authCode *string, declineReason *string, balanceBefore *int64, balanceAfter *int64, processingTimeMs *float64) error {
	cmd, err := r.pool.Exec(ctx, `
		UPDATE issuer_transactions
		SET status = $2,
			response_code = $3,
			auth_code = $4,
			decline_reason = $5,
			balance_before = $6,
			balance_after = $7,
			processing_time_ms = $8,
			updated_at = NOW()
		WHERE id = $1
	`, id, string(status), responseCode, authCode, declineReason, balanceBefore, balanceAfter, processingTimeMs)
	if err != nil {
		return fmt.Errorf("update issuer transaction: %w", err)
	}
	if cmd.RowsAffected() != 1 {
		return fmt.Errorf("update issuer transaction: expected 1 row affected, got %d", cmd.RowsAffected())
	}
	return nil
}
