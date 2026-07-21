-- Drop duplicate PAN index (UNIQUE constraint on cards.pan already creates an implicit unique index)
DROP INDEX IF EXISTS idx_cards_pan;

-- Drop STAN indexes on transaction tables (not queried in hot path, adds write overhead on every INSERT)
DROP INDEX IF EXISTS idx_acq_txn_stan;
DROP INDEX IF EXISTS idx_iss_txn_stan;
