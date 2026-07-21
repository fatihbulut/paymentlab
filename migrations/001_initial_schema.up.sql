CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS cards (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    pan                VARCHAR(19) NOT NULL UNIQUE,
    expiry_date        VARCHAR(4)  NOT NULL,
    card_status        VARCHAR(10) NOT NULL DEFAULT 'ACTIVE',
    scheme             VARCHAR(10) NOT NULL DEFAULT 'GENERIC',
    currency_code      VARCHAR(3)  NOT NULL DEFAULT '949',
    credit_limit       BIGINT      NOT NULL DEFAULT 0,
    available_balance  BIGINT      NOT NULL DEFAULT 0,
    pin_hash           VARCHAR(128),
    cvv_hash           VARCHAR(128),
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

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

CREATE TABLE IF NOT EXISTS issuer_transactions (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    stan               VARCHAR(6)  NOT NULL,
    rrn                VARCHAR(12),
    mti                VARCHAR(4)  NOT NULL,
    pan_masked         VARCHAR(19) NOT NULL,
    amount             BIGINT      NOT NULL,
    currency_code      VARCHAR(3)  NOT NULL,
    status             VARCHAR(20) NOT NULL DEFAULT 'RECEIVED',
    response_code      VARCHAR(2),
    auth_code          VARCHAR(6),
    decline_reason     TEXT,
    balance_before     BIGINT,
    balance_after      BIGINT,
    processing_time_ms DOUBLE PRECISION,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_cards_pan ON cards(pan);
CREATE INDEX IF NOT EXISTS idx_acq_txn_stan ON acquirer_transactions(stan);
CREATE INDEX IF NOT EXISTS idx_iss_txn_stan ON issuer_transactions(stan);
