-- 清空所有实盘数据（用户已确认）
SET FOREIGN_KEY_CHECKS = 0;

TRUNCATE TABLE strategy_cash_flows;
TRUNCATE TABLE run_execution_logs;
TRUNCATE TABLE daily_run_tasks;
TRUNCATE TABLE pre_market_tasks;
TRUNCATE TABLE pre_market_decisions;
TRUNCATE TABLE daily_portfolio_snapshots;
TRUNCATE TABLE live_trades;
TRUNCATE TABLE live_positions;
TRUNCATE TABLE strategy_fund_allocations;
TRUNCATE TABLE strategy_runs;
TRUNCATE TABLE holdings;
TRUNCATE TABLE trade_records;
TRUNCATE TABLE notifications;
TRUNCATE TABLE notification_configs;
TRUNCATE TABLE trading_accounts;

DELETE FROM backtest_signals WHERE strategy_run_id IS NOT NULL;

SET FOREIGN_KEY_CHECKS = 1;

-- 验证
SELECT 'strategy_runs' AS tbl, COUNT(*) AS cnt FROM strategy_runs
UNION ALL SELECT 'live_positions', COUNT(*) FROM live_positions
UNION ALL SELECT 'live_trades', COUNT(*) FROM live_trades
UNION ALL SELECT 'trading_accounts', COUNT(*) FROM trading_accounts
UNION ALL SELECT 'holdings', COUNT(*) FROM holdings
UNION ALL SELECT 'backtest_signals(live)', COUNT(*) FROM backtest_signals WHERE strategy_run_id IS NOT NULL
UNION ALL SELECT 'backtest_signals(backtest)', COUNT(*) FROM backtest_signals WHERE strategy_run_id IS NULL;
