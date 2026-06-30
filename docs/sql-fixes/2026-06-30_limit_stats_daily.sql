-- 2026-06-30: 预计算涨跌停统计表
-- 用于 limit-stats API 加速，替代实时 LATERAL 查询

CREATE TABLE IF NOT EXISTS limit_stats_daily (
    id SERIAL PRIMARY KEY,
    trade_date DATE NOT NULL,
    up_count INT DEFAULT 0,
    down_count INT DEFAULT 0,
    rise_count INT DEFAULT 0,
    fall_count INT DEFAULT 0,
    board_break INT DEFAULT 0,
    total_stocks INT DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_limit_stats_daily_date ON limit_stats_daily(trade_date);

-- 填充示例（由 collect_limit_stats.py 执行）:
-- INSERT INTO limit_stats_daily (trade_date, up_count, down_count, rise_count, fall_count, board_break, total_stocks)
-- VALUES (...) ON CONFLICT (trade_date) DO UPDATE SET ...;
