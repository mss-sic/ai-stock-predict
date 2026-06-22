-- ============================================================
-- 策略条件编排引擎 v2 — 独立修复 SQL
-- 日期: 2026-06-21
-- 功能: 加权评分 / 决策树 / 行业上下文 / AI 代理决策
-- ============================================================

-- ═══ PostgreSQL: ai_agent_decisions 表 ═══
CREATE TABLE IF NOT EXISTS ai_agent_decisions (
    id SERIAL PRIMARY KEY,
    strategy_id INTEGER NOT NULL,
    backtest_task_id INTEGER,
    trade_date VARCHAR(10) NOT NULL,
    market_score NUMERIC(5,2) DEFAULT 0,
    market_bias NUMERIC(5,2) DEFAULT 1.0,
    candidates_in INTEGER DEFAULT 0,
    candidates_out INTEGER DEFAULT 0,
    reasoning TEXT DEFAULT '',
    actions JSONB DEFAULT '[]',
    overrides_applied BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_ai_agent_decisions_strategy ON ai_agent_decisions(strategy_id);
CREATE INDEX IF NOT EXISTS idx_ai_agent_decisions_task ON ai_agent_decisions(backtest_task_id);

-- ═══ MySQL: strategies 新增字段 ═══
ALTER TABLE strategies
    ADD COLUMN IF NOT EXISTS orchestration_mode VARCHAR(20) DEFAULT 'hybrid',
    ADD COLUMN IF NOT EXISTS enable_market_context TINYINT(1) DEFAULT 0,
    ADD COLUMN IF NOT EXISTS market_composite_min DOUBLE DEFAULT -2.0,
    ADD COLUMN IF NOT EXISTS market_position_bias DOUBLE DEFAULT 1.0,
    ADD COLUMN IF NOT EXISTS enable_ai_agent TINYINT(1) DEFAULT 0,
    ADD COLUMN IF NOT EXISTS ai_agent_mode VARCHAR(20) DEFAULT 'advisory',
    ADD COLUMN IF NOT EXISTS ai_agent_review_scope VARCHAR(20) DEFAULT 'all',
    ADD COLUMN IF NOT EXISTS ai_agent_max_daily_trades INTEGER DEFAULT 5,
    ADD COLUMN IF NOT EXISTS industry_filter VARCHAR(500) DEFAULT '',
    ADD COLUMN IF NOT EXISTS enable_sector_rotation TINYINT(1) DEFAULT 0;

-- ═══ MySQL: strategy_conditions 新增字段 ═══
ALTER TABLE strategy_conditions
    ADD COLUMN IF NOT EXISTS weight DOUBLE DEFAULT 1.0,
    ADD COLUMN IF NOT EXISTS fuzzy_sigma DOUBLE DEFAULT 0,
    ADD COLUMN IF NOT EXISTS lookback_days INTEGER DEFAULT 1,
    ADD COLUMN IF NOT EXISTS consecutive_days INTEGER DEFAULT 1,
    ADD COLUMN IF NOT EXISTS trend_direction VARCHAR(10) DEFAULT 'none',
    ADD COLUMN IF NOT EXISTS industry_relative TINYINT(1) DEFAULT 0,
    ADD COLUMN IF NOT EXISTS industry_percentile DOUBLE DEFAULT 50,
    ADD COLUMN IF NOT EXISTS timeframe VARCHAR(10) DEFAULT 'daily',
    ADD COLUMN IF NOT EXISTS parent_id INTEGER DEFAULT NULL,
    ADD COLUMN IF NOT EXISTS composite_type VARCHAR(30) DEFAULT '',
    ADD COLUMN IF NOT EXISTS tree_operator VARCHAR(10) DEFAULT 'and';
CREATE INDEX IF NOT EXISTS idx_sc_parent_id ON strategy_conditions(parent_id);

-- ═══ MySQL: condition_templates 表 ═══
CREATE TABLE IF NOT EXISTS condition_templates (
    id INTEGER PRIMARY KEY AUTO_INCREMENT,
    name VARCHAR(100) NOT NULL,
    description VARCHAR(500) DEFAULT '',
    category VARCHAR(10) DEFAULT 'both',
    cond_type VARCHAR(10) DEFAULT 'buy',
    is_system TINYINT(1) DEFAULT 0,
    created_by INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
