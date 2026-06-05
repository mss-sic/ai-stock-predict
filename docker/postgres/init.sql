-- Enable TimescaleDB
CREATE EXTENSION IF NOT EXISTS timescaledb;

-- 股票基础信息
CREATE TABLE IF NOT EXISTS stocks_basic (
    code        VARCHAR(10) PRIMARY KEY,
    name        VARCHAR(50) NOT NULL,
    industry    VARCHAR(50) DEFAULT '',
    concept_tags JSONB DEFAULT '[]',
    listed_date DATE,
    total_shares BIGINT DEFAULT 0,
    updated_at  TIMESTAMPTZ DEFAULT NOW()
);

-- 日K线 (hypertable)
CREATE TABLE IF NOT EXISTS stocks_daily_k (
    code           VARCHAR(10) NOT NULL,
    trade_date     DATE NOT NULL,
    open           NUMERIC(12,4) DEFAULT 0,
    high           NUMERIC(12,4) DEFAULT 0,
    low            NUMERIC(12,4) DEFAULT 0,
    close          NUMERIC(12,4) DEFAULT 0,
    volume         BIGINT DEFAULT 0,
    amount         NUMERIC(20,2) DEFAULT 0,
    turnover_rate  NUMERIC(10,4) DEFAULT 0,
    PRIMARY KEY (code, trade_date)
);
SELECT create_hypertable('stocks_daily_k', 'trade_date', if_not_exists => TRUE);
CREATE INDEX idx_daily_k_code ON stocks_daily_k (code, trade_date DESC);

-- 日指标 (hypertable)
CREATE TABLE IF NOT EXISTS stocks_daily_indicator (
    code                  VARCHAR(10) NOT NULL,
    trade_date            DATE NOT NULL,
    pe                    NUMERIC(14,4) DEFAULT 0,
    pb                    NUMERIC(14,4) DEFAULT 0,
    ps                    NUMERIC(14,4) DEFAULT 0,
    total_market_cap      NUMERIC(20,2) DEFAULT 0,
    circulating_market_cap NUMERIC(20,2) DEFAULT 0,
    PRIMARY KEY (code, trade_date)
);
SELECT create_hypertable('stocks_daily_indicator', 'trade_date', if_not_exists => TRUE);
CREATE INDEX idx_indicator_code ON stocks_daily_indicator (code, trade_date DESC);

-- 算法精选榜单
CREATE TABLE IF NOT EXISTS algorithm_picks (
    id            SERIAL PRIMARY KEY,
    pick_date     DATE NOT NULL UNIQUE,
    total_stocks  INT DEFAULT 50,
    generated_at  TIMESTAMPTZ DEFAULT NOW()
);

-- 榜单明细
CREATE TABLE IF NOT EXISTS algorithm_pick_details (
    id          SERIAL PRIMARY KEY,
    pick_date   DATE NOT NULL,
    stock_code  VARCHAR(10) NOT NULL,
    rank        INT DEFAULT 0,
    score       NUMERIC(8,2) DEFAULT 0,
    signal_tags JSONB DEFAULT '[]',
    risk_level  VARCHAR(10) DEFAULT 'low',
    suggestion  VARCHAR(10) DEFAULT 'hold',
    UNIQUE (pick_date, stock_code)
);
CREATE INDEX idx_pick_details_date ON algorithm_pick_details (pick_date);
CREATE INDEX idx_pick_details_code ON algorithm_pick_details (stock_code);
