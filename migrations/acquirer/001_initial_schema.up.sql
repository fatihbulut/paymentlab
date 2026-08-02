CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS acquirer_transactions (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    stan          VARCHAR(6)  NOT NULL,
    rrn           VARCHAR(12),
    mti           VARCHAR(4)  NOT NULL,
    pan_masked    VARCHAR(19) NOT NULL,
    amount        BIGINT      NOT NULL,
    currency_code VARCHAR(3)  NOT NULL,
    terminal_id   VARCHAR(8),
    merchant_id   VARCHAR(15),
    status        VARCHAR(20) NOT NULL DEFAULT 'PENDING',
    response_code VARCHAR(2),
    request_hex   TEXT,
    response_hex  TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_acq_txn_stan ON acquirer_transactions(stan);
