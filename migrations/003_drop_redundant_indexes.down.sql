-- Recreate dropped indexes
CREATE INDEX IF NOT EXISTS idx_cards_pan ON cards(pan);
CREATE INDEX IF NOT EXISTS idx_acq_txn_stan ON acquirer_transactions(stan);
CREATE INDEX IF NOT EXISTS idx_iss_txn_stan ON issuer_transactions(stan);
