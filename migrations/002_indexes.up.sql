CREATE INDEX IF NOT EXISTS idx_cards_status ON cards(card_status);
CREATE INDEX IF NOT EXISTS idx_acq_txn_created_at ON acquirer_transactions(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_iss_txn_created_at ON issuer_transactions(created_at DESC);
