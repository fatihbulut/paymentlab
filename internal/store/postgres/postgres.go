package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"iso-parser-service/internal/store"
)

type PostgresStore struct {
	pool      *pgxpool.Pool // Main pool (synchronous_commit=on) for business-critical queries
	auditPool *pgxpool.Pool // Audit pool (synchronous_commit=off) for transaction log writes

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

	// Main pool: synchronous_commit=on (default), for AuthorizeAndDebit
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse pg config: %w", err)
	}

	cfg.MaxConns = 120
	cfg.MinConns = 20
	cfg.MaxConnLifetime = 30 * time.Minute
	cfg.MaxConnIdleTime = 5 * time.Minute
	cfg.HealthCheckPeriod = 30 * time.Second

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create pgx pool: %w", err)
	}

	if err := waitForPool(ctx, pool); err != nil {
		pool.Close()
		return nil, err
	}

	// Audit pool: synchronous_commit=off for faster INSERT throughput
	auditCfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("parse audit pg config: %w", err)
	}

	auditCfg.MaxConns = 30
	auditCfg.MinConns = 5
	auditCfg.MaxConnLifetime = 30 * time.Minute
	auditCfg.MaxConnIdleTime = 5 * time.Minute
	auditCfg.HealthCheckPeriod = 30 * time.Second
	auditCfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		_, err := conn.Exec(ctx, "SET synchronous_commit = off")
		return err
	}

	auditPool, err := pgxpool.NewWithConfig(ctx, auditCfg)
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("create audit pgx pool: %w", err)
	}

	s := &PostgresStore{pool: pool, auditPool: auditPool}
	s.cards = &CardRepository{pool: pool}
	s.acquirerTransactions = &AcquirerTransactionRepository{pool: auditPool}
	s.issuerTransactions = &IssuerTransactionRepository{pool: auditPool}
	return s, nil
}

func waitForPool(ctx context.Context, pool *pgxpool.Pool) error {
	deadline := time.Now().Add(30 * time.Second)
	for {
		pingErr := pool.Ping(ctx)
		if pingErr == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("ping postgres: %w", pingErr)
		}
		time.Sleep(500 * time.Millisecond)
	}
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
	if s.auditPool != nil {
		s.auditPool.Close()
	}
	if s.pool != nil {
		s.pool.Close()
	}
	return nil
}
