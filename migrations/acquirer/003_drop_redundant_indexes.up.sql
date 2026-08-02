-- Drop STAN index (not queried in hot path, adds write overhead on every INSERT)
DROP INDEX IF EXISTS idx_acq_txn_stan;
