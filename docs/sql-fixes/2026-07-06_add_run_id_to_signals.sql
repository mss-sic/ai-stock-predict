-- v060: Add run_id to backtest_signals for multi-run isolation
-- Each live strategy run gets its own signals; backtest signals keep run_id=0

ALTER TABLE backtest_signals ADD COLUMN IF NOT EXISTS run_id INT UNSIGNED NOT NULL DEFAULT 0 AFTER task_id;
CREATE INDEX IF NOT EXISTS idx_sig_run ON backtest_signals(run_id);
