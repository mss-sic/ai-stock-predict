-- 2026-06-19: Market Sentiment System
-- Adds board_type/is_st to stocks_basic + new sentiment tables

-- 1. Add board tracking to stocks_basic
ALTER TABLE stocks_basic ADD COLUMN IF NOT EXISTS board_type VARCHAR(5);
ALTER TABLE stocks_basic ADD COLUMN IF NOT EXISTS is_st BOOLEAN DEFAULT FALSE;

-- Backfill board_type from code prefix
UPDATE stocks_basic SET board_type = 'sh' WHERE code LIKE '60%' AND board_type IS NULL;
UPDATE stocks_basic SET board_type = 'kc' WHERE code LIKE '68%' AND board_type IS NULL;
UPDATE stocks_basic SET board_type = 'sz' WHERE code LIKE '00%' AND board_type IS NULL;
UPDATE stocks_basic SET board_type = 'cy' WHERE code LIKE '30%' AND board_type IS NULL;
UPDATE stocks_basic SET board_type = 'bj' WHERE code ~ '^[89]' AND board_type IS NULL;

-- Backfill is_st from name
UPDATE stocks_basic SET is_st = TRUE WHERE (name LIKE '%ST%' OR name LIKE '%*ST%') AND NOT is_st;

-- 2. market_sentiment — daily composite sentiment
CREATE TABLE IF NOT EXISTS market_sentiment (
    trade_date DATE PRIMARY KEY,
    market_breadth NUMERIC(6,4), breadth_score NUMERIC(5,2),
    style_risk_pref NUMERIC(6,4), style_risk_score NUMERIC(5,2),
    trade_activity NUMERIC(6,4), activity_score NUMERIC(5,2),
    profit_effect NUMERIC(6,4), profit_score NUMERIC(5,2),
    volatility NUMERIC(6,4), vol_score NUMERIC(5,2),
    price_strength NUMERIC(6,4), strength_score NUMERIC(5,2),
    risk_appetite NUMERIC(6,4), risk_app_score NUMERIC(5,2),
    limit_sentiment NUMERIC(6,4), limit_score NUMERIC(5,2),
    sector_diffusion NUMERIC(6,4), sector_score NUMERIC(5,2),
    northbound_net NUMERIC(12,2), northbound_score NUMERIC(5,2),
    capital_flow_net NUMERIC(12,2), capital_flow_score NUMERIC(5,2),
    composite_score NUMERIC(5,2),
    up_count INT DEFAULT 0, down_count INT DEFAULT 0,
    limit_up_count INT DEFAULT 0, limit_down_count INT DEFAULT 0,
    board_break_count INT DEFAULT 0, total_stocks INT DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_market_sentiment_date ON market_sentiment(trade_date DESC);

-- 3. northbound_flow
CREATE TABLE IF NOT EXISTS northbound_flow (
    trade_date DATE PRIMARY KEY,
    hgt_net NUMERIC(12,2), sgt_net NUMERIC(12,2), total_net NUMERIC(12,2),
    hgt_balance NUMERIC(12,2), sgt_balance NUMERIC(12,2),
    created_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_northbound_date ON northbound_flow(trade_date DESC);

-- 4. stock_capital_flow
CREATE TABLE IF NOT EXISTS stock_capital_flow (
    code VARCHAR(10), trade_date DATE,
    main_net NUMERIC(16,2), super_large_net NUMERIC(16,2), large_net NUMERIC(16,2),
    medium_net NUMERIC(16,2), small_net NUMERIC(16,2),
    PRIMARY KEY (code, trade_date)
);
CREATE INDEX IF NOT EXISTS idx_capital_flow_code ON stock_capital_flow(code);
CREATE INDEX IF NOT EXISTS idx_capital_flow_date ON stock_capital_flow(trade_date DESC);

-- 5. sentiment_weights
CREATE TABLE IF NOT EXISTS sentiment_weights (
    id SERIAL PRIMARY KEY, name VARCHAR(50) NOT NULL,
    breadth_w NUMERIC(4,3) DEFAULT 0.0909,
    style_risk_w NUMERIC(4,3) DEFAULT 0.0909,
    activity_w NUMERIC(4,3) DEFAULT 0.0909,
    profit_w NUMERIC(4,3) DEFAULT 0.0909,
    volatility_w NUMERIC(4,3) DEFAULT 0.0909,
    strength_w NUMERIC(4,3) DEFAULT 0.0909,
    risk_appetite_w NUMERIC(4,3) DEFAULT 0.0909,
    limit_w NUMERIC(4,3) DEFAULT 0.0909,
    sector_w NUMERIC(4,3) DEFAULT 0.0909,
    northbound_w NUMERIC(4,3) DEFAULT 0.0909,
    capital_flow_w NUMERIC(4,3) DEFAULT 0.0909,
    is_active BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Seed default equal-weight
INSERT INTO sentiment_weights (name, is_active)
SELECT '等权默认', TRUE
WHERE NOT EXISTS (SELECT 1 FROM sentiment_weights WHERE is_active = TRUE);
