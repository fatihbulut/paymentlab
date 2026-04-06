package postgres

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"iso-parser-service/internal/store"
)

type PostgresStore struct {
	pool      *pgxpool.Pool // Main pool (business-critical queries / migration target)
	auditPool *pgxpool.Pool // Audit pool (synchronous_commit=off) for transaction log writes
	extraPool *pgxpool.Pool // Optional: acquirer uses this to reach issuer DB for card CRUD

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

	// Main pool: synchronous_commit configurable via DB_SYNC_COMMIT (default "on")
	// Set to "off" to eliminate WAL fsync wait on slow cloud disks (~100ms saving per query)
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse pg config: %w", err)
	}

	cfg.MaxConns = int32(envIntOrDefault("DB_POOL_MAX", 120))
	cfg.MinConns = int32(envIntOrDefault("DB_POOL_MIN", 20))
	cfg.MaxConnLifetime = 30 * time.Minute
	cfg.MaxConnIdleTime = 5 * time.Minute
	cfg.HealthCheckPeriod = 30 * time.Second

	syncCommit := os.Getenv("DB_SYNC_COMMIT")
	if syncCommit == "off" {
		cfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
			_, err := conn.Exec(ctx, "SET synchronous_commit = off")
			return err
		}
	}

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

	auditCfg.MaxConns = int32(envIntOrDefault("DB_AUDIT_POOL_MAX", 30))
	auditCfg.MinConns = int32(envIntOrDefault("DB_AUDIT_POOL_MIN", 5))
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

func envIntOrDefault(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}

// NewAcquirerStore creates a store for the acquirer service with two separate DB connections:
//   - acquirerURL: acquirer's own DB, used for audit transaction logs
//   - issuerURL:   issuer's DB, used only for card CRUD (create/list/update/delete)
func NewAcquirerStore(ctx context.Context, acquirerURL, issuerURL string) (*PostgresStore, error) {
	if acquirerURL == "" {
		return nil, fmt.Errorf("acquirerURL is empty")
	}
	if issuerURL == "" {
		return nil, fmt.Errorf("issuerURL is empty")
	}

	// Acquirer's own pool — used for Pool() (migration target) and future acquirer-side queries
	acqCfg, err := pgxpool.ParseConfig(acquirerURL)
	if err != nil {
		return nil, fmt.Errorf("parse acquirer pg config: %w", err)
	}
	acqCfg.MaxConns = int32(envIntOrDefault("DB_POOL_MAX", 10))
	acqCfg.MinConns = int32(envIntOrDefault("DB_POOL_MIN", 2))
	acqCfg.MaxConnLifetime = 30 * time.Minute
	acqCfg.MaxConnIdleTime = 5 * time.Minute
	acqCfg.HealthCheckPeriod = 30 * time.Second

	acqPool, err := pgxpool.NewWithConfig(ctx, acqCfg)
	if err != nil {
		return nil, fmt.Errorf("create acquirer pool: %w", err)
	}
	if err := waitForPool(ctx, acqPool); err != nil {
		acqPool.Close()
		return nil, err
	}

	// Acquirer audit pool (synchronous_commit=off) for AcquirerTransaction INSERTs
	auditCfg, err := pgxpool.ParseConfig(acquirerURL)
	if err != nil {
		acqPool.Close()
		return nil, fmt.Errorf("parse acquirer audit pg config: %w", err)
	}
	auditCfg.MaxConns = int32(envIntOrDefault("DB_AUDIT_POOL_MAX", 10))
	auditCfg.MinConns = int32(envIntOrDefault("DB_AUDIT_POOL_MIN", 2))
	auditCfg.MaxConnLifetime = 30 * time.Minute
	auditCfg.MaxConnIdleTime = 5 * time.Minute
	auditCfg.HealthCheckPeriod = 30 * time.Second
	auditCfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		_, err := conn.Exec(ctx, "SET synchronous_commit = off")
		return err
	}

	auditPool, err := pgxpool.NewWithConfig(ctx, auditCfg)
	if err != nil {
		acqPool.Close()
		return nil, fmt.Errorf("create acquirer audit pool: %w", err)
	}

	// Issuer DB pool — small, only for card admin operations (low-volume)
	issuerCfg, err := pgxpool.ParseConfig(issuerURL)
	if err != nil {
		acqPool.Close()
		auditPool.Close()
		return nil, fmt.Errorf("parse issuer pg config: %w", err)
	}
	issuerCfg.MaxConns = int32(envIntOrDefault("ISSUER_DB_POOL_MAX", 5))
	issuerCfg.MinConns = int32(envIntOrDefault("ISSUER_DB_POOL_MIN", 1))
	issuerCfg.MaxConnLifetime = 30 * time.Minute
	issuerCfg.MaxConnIdleTime = 5 * time.Minute
	issuerCfg.HealthCheckPeriod = 30 * time.Second

	issuerPool, err := pgxpool.NewWithConfig(ctx, issuerCfg)
	if err != nil {
		acqPool.Close()
		auditPool.Close()
		return nil, fmt.Errorf("create issuer pool: %w", err)
	}
	if err := waitForPool(ctx, issuerPool); err != nil {
		acqPool.Close()
		auditPool.Close()
		issuerPool.Close()
		return nil, err
	}

	s := &PostgresStore{pool: acqPool, auditPool: auditPool, extraPool: issuerPool}
	s.cards = &CardRepository{pool: issuerPool}                              // card CRUD → issuer DB
	s.acquirerTransactions = &AcquirerTransactionRepository{pool: auditPool} // audit → acquirer DB
	// s.issuerTransactions intentionally nil — acquirer does not write issuer TX logs
	return s, nil
}

func (s *PostgresStore) Close() error {
	if s.extraPool != nil {
		s.extraPool.Close()
	}
	if s.auditPool != nil {
		s.auditPool.Close()
	}
	if s.pool != nil {
		s.pool.Close()
	}
	return nil
}
