package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"iso-parser-service/internal/store"
)

type PostgresStore struct {
	pool *pgxpool.Pool

	cards                *CardRepository
	acquirerTransactions *AcquirerTransactionRepository
	issuerTransactions   *IssuerTransactionRepository
}

func (s *PostgresStore) Pool() *pgxpool.Pool {
	return s.pool
}

func New(ctx context.Context, databaseURL string) (*PostgresStore, error) {
	if databaseURL == "" {
		return nil, fmt.Errorf("databaseURL is empty")
	}

	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse pg config: %w", err)
	}

	cfg.MaxConns = 50
	cfg.MinConns = 5
	cfg.MaxConnLifetime = 30 * time.Minute
	cfg.MaxConnIdleTime = 5 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create pgx pool: %w", err)
	}

	deadline := time.Now().Add(30 * time.Second)
	for {
		pingErr := pool.Ping(ctx)
		if pingErr == nil {
			break
		}
		if time.Now().After(deadline) {
			pool.Close()
			return nil, fmt.Errorf("ping postgres: %w", pingErr)
		}
		time.Sleep(500 * time.Millisecond)
	}

	s := &PostgresStore{pool: pool}
	s.cards = &CardRepository{pool: pool}
	s.acquirerTransactions = &AcquirerTransactionRepository{pool: pool}
	s.issuerTransactions = &IssuerTransactionRepository{pool: pool}
	return s, nil
}

func (s *PostgresStore) Cards() store.CardStore {
	return s.cards
}

func (s *PostgresStore) AcquirerTransactions() store.AcquirerTransactionStore {
	return s.acquirerTransactions
}

func (s *PostgresStore) IssuerTransactions() store.IssuerTransactionStore {
	return s.issuerTransactions
}

func (s *PostgresStore) Close() error {
	if s.pool != nil {
		s.pool.Close()
	}
	return nil
}
