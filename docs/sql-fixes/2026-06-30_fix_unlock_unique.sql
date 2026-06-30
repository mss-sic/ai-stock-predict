-- Fix: restricted_share_unlock table missing unique constraint for ON CONFLICT
CREATE UNIQUE INDEX IF NOT EXISTS idx_restricted_unlock_unique ON restricted_share_unlock (code, free_date, stock_type);
