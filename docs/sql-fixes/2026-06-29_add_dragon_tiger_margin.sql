-- 2026-06-29: 龙虎榜 + 融资融券 数据表
-- 对应迁移: v037

CREATE TABLE IF NOT EXISTS dragon_tiger_list (
    id SERIAL PRIMARY KEY,
    code VARCHAR(10),
    name VARCHAR(50),
    trade_date DATE,
    reason VARCHAR(200),
    close_price NUMERIC(12,4),
    change_pct NUMERIC(8,4),
    net_buy_amt NUMERIC(16,2),
    buy_amt NUMERIC(16,2),
    sell_amt NUMERIC(16,2),
    turnover_pct NUMERIC(8,4),
    created_at TIMESTAMPTZ DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_dragon_tiger_list_code ON dragon_tiger_list(code);
CREATE INDEX IF NOT EXISTS idx_dragon_tiger_list_date ON dragon_tiger_list(trade_date);
CREATE UNIQUE INDEX IF NOT EXISTS idx_dragon_tiger_list_code_date ON dragon_tiger_list(code, trade_date);

CREATE TABLE IF NOT EXISTS dragon_tiger_detail (
    id SERIAL PRIMARY KEY,
    code VARCHAR(10),
    trade_date DATE,
    seat_name VARCHAR(100),
    seat_code VARCHAR(20),
    side VARCHAR(5),
    buy_amt NUMERIC(16,2),
    sell_amt NUMERIC(16,2),
    net_amt NUMERIC(16,2),
    is_institution BOOLEAN DEFAULT false,
    created_at TIMESTAMPTZ DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_dragon_tiger_detail_code ON dragon_tiger_detail(code);
CREATE INDEX IF NOT EXISTS idx_dragon_tiger_detail_date ON dragon_tiger_detail(trade_date);

CREATE TABLE IF NOT EXISTS margin_trading (
    id SERIAL PRIMARY KEY,
    code VARCHAR(10),
    trade_date DATE,
    rzye NUMERIC(24,2),
    rzmre NUMERIC(24,2),
    rzche NUMERIC(24,2),
    rqye NUMERIC(24,2),
    rqmcl NUMERIC(24,2),
    rqchl NUMERIC(24,2),
    rzrqye NUMERIC(24,2),
    created_at TIMESTAMPTZ DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_margin_trading_code ON margin_trading(code);
CREATE UNIQUE INDEX IF NOT EXISTS idx_margin_trading_code_date ON margin_trading(code, trade_date);
