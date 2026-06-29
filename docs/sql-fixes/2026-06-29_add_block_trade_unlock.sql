-- 2026-06-29: 大宗交易 + 限售解禁 数据表
-- 对应迁移: v038

CREATE TABLE IF NOT EXISTS block_trade (
    id SERIAL PRIMARY KEY,
    code VARCHAR(10),
    trade_date DATE,
    deal_price NUMERIC(12,4),
    close_price NUMERIC(12,4),
    premium_pct NUMERIC(8,4),
    deal_volume NUMERIC(24,2),
    deal_amt NUMERIC(24,2),
    buyer_name VARCHAR(100),
    seller_name VARCHAR(100),
    created_at TIMESTAMPTZ DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_block_trade_code ON block_trade(code);
CREATE INDEX IF NOT EXISTS idx_block_trade_date ON block_trade(trade_date);

CREATE TABLE IF NOT EXISTS restricted_share_unlock (
    id SERIAL PRIMARY KEY,
    code VARCHAR(10),
    free_date DATE,
    stock_type VARCHAR(100),
    shares NUMERIC(24,2),
    ratio NUMERIC(12,4),
    is_history BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_restricted_unlock_code ON restricted_share_unlock(code);
CREATE INDEX IF NOT EXISTS idx_restricted_unlock_date ON restricted_share_unlock(free_date);
