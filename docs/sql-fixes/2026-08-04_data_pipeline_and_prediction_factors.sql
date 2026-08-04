-- v097-v101 production repair SQL.
-- Covers trade calendar, K-line metadata, cash-flow fields, indicator cache,
-- and prediction factor precomputation. Statements are idempotent.

CREATE TABLE IF NOT EXISTS trade_calendar (
    trade_date DATE PRIMARY KEY,
    is_trading_day BOOLEAN DEFAULT true,
    holiday_name VARCHAR(50),
    data_source VARCHAR(20) DEFAULT 'tushare',
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_trade_calendar_date ON trade_calendar(trade_date);
CREATE INDEX IF NOT EXISTS idx_trade_calendar_trading ON trade_calendar(is_trading_day, trade_date);

ALTER TABLE stocks_daily_k ADD COLUMN IF NOT EXISTS adj_factor NUMERIC(12,8) DEFAULT 1.0;
ALTER TABLE stocks_daily_k ADD COLUMN IF NOT EXISTS source_priority INT DEFAULT 0;
ALTER TABLE stocks_daily_k ADD COLUMN IF NOT EXISTS data_quality VARCHAR(20) DEFAULT 'ok';
ALTER TABLE stocks_daily_k ADD COLUMN IF NOT EXISTS ema12 NUMERIC(12,4);
ALTER TABLE stocks_daily_k ADD COLUMN IF NOT EXISTS ema26 NUMERIC(12,4);
ALTER TABLE stocks_daily_k ADD COLUMN IF NOT EXISTS macd_bar NUMERIC(12,4);

ALTER TABLE stock_financials ADD COLUMN IF NOT EXISTS operating_cf NUMERIC(20,2);
ALTER TABLE stock_financials ADD COLUMN IF NOT EXISTS investing_cf NUMERIC(20,2);
ALTER TABLE stock_financials ADD COLUMN IF NOT EXISTS financing_cf NUMERIC(20,2);
ALTER TABLE stock_financials ADD COLUMN IF NOT EXISTS net_cash_flow NUMERIC(20,2);
ALTER TABLE stock_financials ADD COLUMN IF NOT EXISTS free_cf NUMERIC(20,2);
ALTER TABLE stock_financials ADD COLUMN IF NOT EXISTS cf_ratio NUMERIC(10,4);

CREATE TABLE IF NOT EXISTS stock_daily_indicators (
    code VARCHAR(10) NOT NULL,
    trade_date DATE NOT NULL,
    daily_change NUMERIC(8,4), pe NUMERIC(12,4), pb NUMERIC(12,4),
    rsi NUMERIC(8,2), volume_ratio NUMERIC(8,4), turnover_rate NUMERIC(8,4),
    total_market_cap NUMERIC(20,2), algo_score NUMERIC(8,2),
    indicators JSONB NOT NULL DEFAULT '{}',
    adj_factor NUMERIC(12,8) DEFAULT 1.0,
    data_quality VARCHAR(10) DEFAULT 'ok',
    computed_at TIMESTAMP DEFAULT NOW(),
    PRIMARY KEY (code, trade_date)
);
CREATE INDEX IF NOT EXISTS idx_sdi_date ON stock_daily_indicators(trade_date);
CREATE INDEX IF NOT EXISTS idx_sdi_pe ON stock_daily_indicators(trade_date, pe) WHERE pe > 0;
CREATE INDEX IF NOT EXISTS idx_sdi_rsi ON stock_daily_indicators(trade_date, rsi);
CREATE INDEX IF NOT EXISTS idx_sdi_change ON stock_daily_indicators(trade_date, daily_change);

CREATE TABLE IF NOT EXISTS prediction_factors (
    code VARCHAR(10) PRIMARY KEY,
    consensus_d5 INT NOT NULL DEFAULT 0,
    exp_return_d5 NUMERIC(10,6) NOT NULL DEFAULT 0,
    momentum_d5 NUMERIC(10,6) NOT NULL DEFAULT 0,
    consensus_d10 INT NOT NULL DEFAULT 0,
    exp_return_d10 NUMERIC(10,6) NOT NULL DEFAULT 0,
    momentum_d10 NUMERIC(10,6) NOT NULL DEFAULT 0,
    consensus_d20 INT NOT NULL DEFAULT 0,
    exp_return_d20 NUMERIC(10,6) NOT NULL DEFAULT 0,
    momentum_d20 NUMERIC(10,6) NOT NULL DEFAULT 0,
    stddev_d20 NUMERIC(10,6) NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

WITH curves AS (
    SELECT pk.code, pk.updated_at, curve
    FROM prediction_kdist pk
    CROSS JOIN LATERAL jsonb_array_elements(COALESCE(pk.kd_data, '[]'::jsonb)) AS curve
    WHERE jsonb_typeof(curve) = 'array' AND jsonb_array_length(curve) >= 20
), values_by_curve AS (
    SELECT code, updated_at,
        (curve->>4)::numeric AS d5, (curve->>9)::numeric AS d10, (curve->>19)::numeric AS d20,
        (((curve->>15)::numeric + (curve->>16)::numeric + (curve->>17)::numeric +
          (curve->>18)::numeric + (curve->>19)::numeric) / 5 -
         ((curve->>0)::numeric + (curve->>1)::numeric + (curve->>2)::numeric +
          (curve->>3)::numeric + (curve->>4)::numeric) / 5) AS momentum
    FROM curves
), aggregated AS (
    SELECT code,
        COUNT(*) FILTER (WHERE d5 > 0)::int AS consensus_d5,
        AVG(d5) AS exp_return_d5, AVG(momentum) AS momentum_d5,
        COUNT(*) FILTER (WHERE d10 > 0)::int AS consensus_d10,
        AVG(d10) AS exp_return_d10, AVG(momentum) AS momentum_d10,
        COUNT(*) FILTER (WHERE d20 > 0)::int AS consensus_d20,
        AVG(d20) AS exp_return_d20, AVG(momentum) AS momentum_d20,
        COALESCE(STDDEV_POP(d20), 0) AS stddev_d20, MAX(updated_at) AS updated_at
    FROM values_by_curve GROUP BY code
)
INSERT INTO prediction_factors (code, consensus_d5, exp_return_d5, momentum_d5,
    consensus_d10, exp_return_d10, momentum_d10,
    consensus_d20, exp_return_d20, momentum_d20, stddev_d20, updated_at)
SELECT code, consensus_d5, exp_return_d5, momentum_d5,
    consensus_d10, exp_return_d10, momentum_d10,
    consensus_d20, exp_return_d20, momentum_d20, stddev_d20, updated_at
FROM aggregated
ON CONFLICT (code) DO UPDATE SET
    consensus_d5 = EXCLUDED.consensus_d5, exp_return_d5 = EXCLUDED.exp_return_d5,
    momentum_d5 = EXCLUDED.momentum_d5, consensus_d10 = EXCLUDED.consensus_d10,
    exp_return_d10 = EXCLUDED.exp_return_d10, momentum_d10 = EXCLUDED.momentum_d10,
    consensus_d20 = EXCLUDED.consensus_d20, exp_return_d20 = EXCLUDED.exp_return_d20,
    momentum_d20 = EXCLUDED.momentum_d20, stddev_d20 = EXCLUDED.stddev_d20,
    updated_at = EXCLUDED.updated_at;
