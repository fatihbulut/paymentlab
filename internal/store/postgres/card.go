package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"iso-parser-service/internal/store"
)

type CardRepository struct {
	pool *pgxpool.Pool
}

func (r *CardRepository) GetCardByID(ctx context.Context, id string) (*store.Card, error) {
	var c store.Card
	row := r.pool.QueryRow(ctx, `
		SELECT id, pan, expiry_date, card_status, scheme, currency_code,
			credit_limit, available_balance, pin_hash, cvv_hash,
			created_at, updated_at
		FROM cards
		WHERE id = $1
	`, id)

	var status string
	if err := row.Scan(
		&c.ID,
		&c.PAN,
		&c.ExpiryDate,
		&status,
		&c.Scheme,
		&c.CurrencyCode,
		&c.CreditLimit,
		&c.AvailableBalance,
		&c.PinHash,
		&c.CvvHash,
		&c.CreatedAt,
		&c.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("select card by id: %w", err)
	}

	c.Status = store.CardStatus(status)
	return &c, nil
}

func (r *CardRepository) CreateCard(ctx context.Context, c *store.Card) (*store.Card, error) {
	if c == nil {
		return nil, fmt.Errorf("card is nil")
	}

	row := r.pool.QueryRow(ctx, `
		INSERT INTO cards (
			pan, expiry_date, card_status, scheme, currency_code,
			credit_limit, available_balance, pin_hash, cvv_hash
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		RETURNING id, created_at, updated_at
	`, c.PAN, c.ExpiryDate, string(c.Status), c.Scheme, c.CurrencyCode, c.CreditLimit, c.AvailableBalance, c.PinHash, c.CvvHash)

	if err := row.Scan(&c.ID, &c.CreatedAt, &c.UpdatedAt); err != nil {
		return nil, fmt.Errorf("insert card: %w", err)
	}

	return c, nil
}

func (r *CardRepository) GetCardByPAN(ctx context.Context, pan string) (*store.Card, error) {
	var c store.Card
	row := r.pool.QueryRow(ctx, `
		SELECT id, pan, expiry_date, card_status, scheme, currency_code,
			credit_limit, available_balance, pin_hash, cvv_hash,
			created_at, updated_at
		FROM cards
		WHERE pan = $1
	`, pan)

	var status string
	if err := row.Scan(
		&c.ID,
		&c.PAN,
		&c.ExpiryDate,
		&status,
		&c.Scheme,
		&c.CurrencyCode,
		&c.CreditLimit,
		&c.AvailableBalance,
		&c.PinHash,
		&c.CvvHash,
		&c.CreatedAt,
		&c.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("select card: %w", err)
	}

	c.Status = store.CardStatus(status)
	return &c, nil
}

func (r *CardRepository) ListCards(ctx context.Context, limit int, offset int) ([]store.Card, error) {
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
		SELECT id, pan, expiry_date, card_status, scheme, currency_code,
			credit_limit, available_balance, pin_hash, cvv_hash,
			created_at, updated_at
		FROM cards
		WHERE card_status != 'DELETED'
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list cards: %w", err)
	}
	defer rows.Close()

	result := make([]store.Card, 0, limit)
	for rows.Next() {
		var c store.Card
		var status string
		if err := rows.Scan(
			&c.ID,
			&c.PAN,
			&c.ExpiryDate,
			&status,
			&c.Scheme,
			&c.CurrencyCode,
			&c.CreditLimit,
			&c.AvailableBalance,
			&c.PinHash,
			&c.CvvHash,
			&c.CreatedAt,
			&c.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan card: %w", err)
		}
		c.Status = store.CardStatus(status)
		result = append(result, c)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list cards rows: %w", err)
	}

	return result, nil
}

func (r *CardRepository) UpdateCard(ctx context.Context, c *store.Card) (*store.Card, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE cards
		SET expiry_date       = $2,
		    scheme            = $3,
		    currency_code     = $4,
		    credit_limit      = $5,
		    available_balance = $6,
		    card_status       = $7,
		    updated_at        = NOW()
		WHERE id = $1
		RETURNING updated_at
	`, c.ID, c.ExpiryDate, c.Scheme, c.CurrencyCode, c.CreditLimit, c.AvailableBalance, string(c.Status))
	if err := row.Scan(&c.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("card not found")
		}
		return nil, fmt.Errorf("update card: %w", err)
	}
	return c, nil
}

func (r *CardRepository) SoftDeleteCard(ctx context.Context, id string) error {
	cmd, err := r.pool.Exec(ctx, `
		UPDATE cards SET card_status = 'DELETED', updated_at = NOW() WHERE id = $1
	`, id)
	if err != nil {
		return fmt.Errorf("soft delete card: %w", err)
	}
	if cmd.RowsAffected() != 1 {
		return fmt.Errorf("card not found")
	}
	return nil
}

func (r *CardRepository) CreditBalance(ctx context.Context, id string, amount int64) error {
	if amount <= 0 {
		return fmt.Errorf("amount must be > 0")
	}
	cmd, err := r.pool.Exec(ctx, `
		UPDATE cards
		SET available_balance = available_balance + $2, updated_at = NOW()
		WHERE id = $1
	`, id, amount)
	if err != nil {
		return fmt.Errorf("credit balance: %w", err)
	}
	if cmd.RowsAffected() != 1 {
		return fmt.Errorf("credit balance: card not found")
	}
	return nil
}

func (r *CardRepository) UpdateCardBalance(ctx context.Context, id string, newAvailableBalance int64) error {
	cmd, err := r.pool.Exec(ctx, `
		UPDATE cards
		SET available_balance = $2, updated_at = NOW()
		WHERE id = $1
	`, id, newAvailableBalance)
	if err != nil {
		return fmt.Errorf("update balance: %w", err)
	}
	if cmd.RowsAffected() != 1 {
		return fmt.Errorf("update balance: expected 1 row affected, got %d", cmd.RowsAffected())
	}
	return nil
}

func (r *CardRepository) DebitIfSufficient(ctx context.Context, id string, amount int64) (before int64, after int64, ok bool, err error) {
	if amount <= 0 {
		return 0, 0, false, fmt.Errorf("amount must be > 0")
	}

	row := r.pool.QueryRow(ctx, `
		UPDATE cards
		SET available_balance = available_balance - $2,
			updated_at = NOW()
		WHERE id = $1
			AND available_balance >= $2
		RETURNING available_balance + $2 AS balance_before,
			available_balance AS balance_after
	`, id, amount)

	if scanErr := row.Scan(&before, &after); scanErr != nil {
		if errors.Is(scanErr, pgx.ErrNoRows) {
			return 0, 0, false, nil
		}
		return 0, 0, false, fmt.Errorf("debit: %w", scanErr)
	}

	return before, after, true, nil
}
