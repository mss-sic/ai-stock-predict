-- 2026-06-22: Add Policy Manager v3 columns to strategies table (v027 migration)
-- Run this if gormAutoMigrate doesn't handle JSONMap type correctly

ALTER TABLE strategies
  ADD COLUMN IF NOT EXISTS policy_mode VARCHAR(20) DEFAULT 'rule',
  ADD COLUMN IF NOT EXISTS aggressive_threshold DOUBLE DEFAULT 1.5,
  ADD COLUMN IF NOT EXISTS defensive_threshold DOUBLE DEFAULT 0.0,
  ADD COLUMN IF NOT EXISTS policy_aggressive JSON,
  ADD COLUMN IF NOT EXISTS policy_defensive JSON,
  ADD COLUMN IF NOT EXISTS policy_cash JSON;

-- NOTE: MySQL 5.7 does NOT support "ADD COLUMN IF NOT EXISTS".
-- Use the following fallback if the above fails:

/*
-- Check which columns exist first:
DESCRIBE strategies;

-- Then add only missing ones:
ALTER TABLE strategies ADD COLUMN policy_mode VARCHAR(20) DEFAULT 'rule';
ALTER TABLE strategies ADD COLUMN aggressive_threshold DOUBLE DEFAULT 1.5;
ALTER TABLE strategies ADD COLUMN defensive_threshold DOUBLE DEFAULT 0.0;
ALTER TABLE strategies ADD COLUMN policy_aggressive JSON;
ALTER TABLE strategies ADD COLUMN policy_defensive JSON;
ALTER TABLE strategies ADD COLUMN policy_cash JSON;
*/
