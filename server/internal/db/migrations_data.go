package db

import (
	"log"

	"github.com/ai-stock-predict/server/internal/model"
)

func init() {
	// ============================================================
	// v001: PG concept tables + stock_signals
	// ============================================================
	Register(Migration{
		Version:     1,
		Description: "PG: concept_boards, stock_concepts, stock_signals",
		Up: func() error {
			safeExec(`CREATE TABLE IF NOT EXISTS concept_boards (
				concept_code VARCHAR(20) PRIMARY KEY,
				concept_name VARCHAR(100) NOT NULL,
				concept_type VARCHAR(20) DEFAULT 'concept',
				stock_count INT DEFAULT 0,
				updated_at TIMESTAMPTZ DEFAULT NOW()
			)`)
			safeExec(`CREATE INDEX IF NOT EXISTS idx_concept_boards_name ON concept_boards(concept_name)`)

			safeExec(`CREATE TABLE IF NOT EXISTS stock_concepts (
				id SERIAL PRIMARY KEY,
				code VARCHAR(10) NOT NULL,
				concept_code VARCHAR(20) NOT NULL,
				concept_name VARCHAR(100) NOT NULL,
				concept_type VARCHAR(20) DEFAULT 'concept',
				stock_name VARCHAR(50),
				updated_at TIMESTAMPTZ DEFAULT NOW(),
				UNIQUE(code, concept_code)
			)`)
			safeExec(`CREATE INDEX IF NOT EXISTS idx_stock_concepts_code ON stock_concepts(code)`)
			safeExec(`CREATE INDEX IF NOT EXISTS idx_stock_concepts_concept ON stock_concepts(concept_code)`)
			safeExec(`CREATE INDEX IF NOT EXISTS idx_stock_concepts_name ON stock_concepts(concept_name)`)

			safeExec(`CREATE TABLE IF NOT EXISTS stock_signals (
				id SERIAL PRIMARY KEY,
				code VARCHAR(10) UNIQUE,
				signal_value NUMERIC(12,6),
				source VARCHAR(50) DEFAULT 'excel_import',
				updated_at TIMESTAMPTZ DEFAULT NOW()
			)`)
			return nil
		},
	})

	// ============================================================
	// v002: PG GORM AutoMigrate (all PG models)
	// ============================================================
	Register(Migration{
		Version:     2,
		Description: "PG: GORM AutoMigrate stock/indicator/prediction models",
		Up: func() error {
			gormAutoMigrate(PG,
				&model.StockBasic{},
				&model.StockDailyK{},
				&model.StockDailyIndicator{},
				&model.AlgorithmPick{},
				&model.AlgorithmPickDetail{},
				&model.StockSignal{},
				&model.AIAnalysis{},
				&model.Prediction{},
				&model.AIConversation{},
				&model.AIStockScore{},
				&model.StockShareholder{},
				&model.StockFinancial{},
				&model.StockNews{},
				&model.ConceptBoard{},
				&model.StockConcept{},
				&model.PredictionKDist{},
			)
			return nil
		},
	})

	// ============================================================
	// v003: PG ai_conversations, predictions, ai_analyses, ai_stock_scores
	// ============================================================
	Register(Migration{
		Version:     3,
		Description: "PG: ai_conversations, predictions, ai_analyses, ai_stock_scores (manual)",
		Up: func() error {
			safeExec(`CREATE TABLE IF NOT EXISTS ai_conversations (
				id SERIAL PRIMARY KEY,
				code VARCHAR(10),
				role VARCHAR(10),
				content TEXT,
				created_at TIMESTAMPTZ DEFAULT NOW()
			)`)
			safeExec(`CREATE INDEX IF NOT EXISTS idx_ai_conv_code ON ai_conversations(code)`)
			safeExec(`CREATE TABLE IF NOT EXISTS predictions (
				id SERIAL PRIMARY KEY,
				code VARCHAR(10),
				model_name VARCHAR(30),
				predict_date DATE,
				predicted_price NUMERIC(12,4),
				upper_bound NUMERIC(12,4),
				lower_bound NUMERIC(12,4),
				created_at TIMESTAMPTZ DEFAULT NOW()
			)`)
			safeExec(`CREATE INDEX IF NOT EXISTS idx_pred_code ON predictions(code)`)
			safeExec(`CREATE TABLE IF NOT EXISTS ai_analyses (
				id SERIAL PRIMARY KEY,
				code VARCHAR(10),
				pick_date VARCHAR(10),
				model VARCHAR(50),
				risk_level VARCHAR(20),
				suggestion VARCHAR(20),
				summary TEXT,
				signals JSONB,
				created_at TIMESTAMPTZ DEFAULT NOW()
			)`)
			safeExec(`CREATE INDEX IF NOT EXISTS idx_ai_analysis_code ON ai_analyses(code)`)
			safeExec(`CREATE TABLE IF NOT EXISTS ai_stock_scores (
				id SERIAL PRIMARY KEY,
				code VARCHAR(10),
				composite_score NUMERIC(4,2),
				fundamental_score NUMERIC(4,2),
				growth_score NUMERIC(4,2),
				valuation_score NUMERIC(4,2),
				capital_score NUMERIC(4,2),
				technical_score NUMERIC(4,2),
				industry_score NUMERIC(4,2),
				risk_level VARCHAR(20),
				suggestion VARCHAR(20),
				risk_warnings JSONB,
				summary TEXT,
				analyzed_at TIMESTAMPTZ DEFAULT NOW(),
				created_at TIMESTAMPTZ DEFAULT NOW()
			)`)
			safeExec(`CREATE INDEX IF NOT EXISTS idx_ai_score_code ON ai_stock_scores(code)`)
			return nil
		},
	})

	// ============================================================
	// v004: PG stock_shareholders, stock_financials, stock_news
	// ============================================================
	Register(Migration{
		Version:     4,
		Description: "PG: stock_shareholders, stock_financials, stock_news",
		Up: func() error {
			safeExec(`CREATE TABLE IF NOT EXISTS stock_shareholders (
				id SERIAL PRIMARY KEY, code VARCHAR(10), report_date VARCHAR(10),
				total_holders BIGINT, holder_change NUMERIC(10,4),
				top10_holders JSONB, top10_float JSONB,
				inst_hold_ratio NUMERIC(10,4), avg_holding BIGINT,
				created_at TIMESTAMPTZ DEFAULT NOW()
			)`)
			safeExec(`DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='uq_shareholders') THEN ALTER TABLE stock_shareholders ADD CONSTRAINT uq_shareholders UNIQUE (code, report_date); END IF; END $$`)
			safeExec(`CREATE TABLE IF NOT EXISTS stock_financials (
				id SERIAL PRIMARY KEY, code VARCHAR(10), report_date VARCHAR(10),
				report_type VARCHAR(10), total_revenue NUMERIC(20,2), net_profit NUMERIC(20,2),
				revenue_growth NUMERIC(10,4), profit_growth NUMERIC(10,4),
				total_assets NUMERIC(20,2), total_liabilities NUMERIC(20,2), net_assets NUMERIC(20,2),
				roe NUMERIC(10,4), eps NUMERIC(10,4), bps NUMERIC(10,4),
				gross_margin NUMERIC(10,4), net_margin NUMERIC(10,4), debt_ratio NUMERIC(10,4),
				created_at TIMESTAMPTZ DEFAULT NOW()
			)`)
			safeExec(`DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='uq_financials') THEN ALTER TABLE stock_financials ADD CONSTRAINT uq_financials UNIQUE (code, report_date); END IF; END $$`)
			safeExec(`CREATE TABLE IF NOT EXISTS stock_news (
				id SERIAL PRIMARY KEY, code VARCHAR(10), title VARCHAR(500),
				summary TEXT, source VARCHAR(50), news_type VARCHAR(20),
				url VARCHAR(500), publish_date VARCHAR(10),
				created_at TIMESTAMPTZ DEFAULT NOW()
			)`)
			safeExec(`CREATE INDEX IF NOT EXISTS idx_news_code ON stock_news(code)`)
			safeExec(`DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='uq_news') THEN ALTER TABLE stock_news ADD CONSTRAINT uq_news UNIQUE (code, title, publish_date); END IF; END $$`)
			return nil
		},
	})

	// ============================================================
	// v005: PG stock_reports, prediction_kdist
	// ============================================================
	Register(Migration{
		Version:     5,
		Description: "PG: stock_reports, prediction_kdist",
		Up: func() error {
			safeExec(`CREATE TABLE IF NOT EXISTS stock_reports (
				id SERIAL PRIMARY KEY, info_code VARCHAR(30) UNIQUE,
				title VARCHAR(500), stock_code VARCHAR(10), stock_name VARCHAR(50),
				org_name VARCHAR(200), org_sname VARCHAR(100),
				publish_date VARCHAR(10), rating VARCHAR(20), rating_change VARCHAR(20),
				predict_this_year_eps NUMERIC(12,4), predict_this_year_pe NUMERIC(12,4),
				predict_next_year_eps NUMERIC(12,4), predict_next_year_pe NUMERIC(12,4),
				predict_next_two_year_eps NUMERIC(12,4), predict_next_two_year_pe NUMERIC(12,4),
				author JSONB, researcher VARCHAR(200), industry_name VARCHAR(100),
				pdf_url VARCHAR(200), attach_size INTEGER, attach_pages INTEGER,
				created_at TIMESTAMPTZ DEFAULT NOW()
			)`)
			safeExec(`CREATE INDEX IF NOT EXISTS idx_reports_code ON stock_reports(stock_code)`)
			safeExec(`CREATE INDEX IF NOT EXISTS idx_reports_date ON stock_reports(publish_date)`)
			safeExec(`CREATE INDEX IF NOT EXISTS idx_reports_industry ON stock_reports(industry_name)`)

			safeExec(`CREATE TABLE IF NOT EXISTS prediction_kdist (
				id SERIAL PRIMARY KEY,
				code VARCHAR(10) UNIQUE,
				kd_data JSONB,
				updated_at TIMESTAMPTZ DEFAULT NOW()
			)`)
			return nil
		},
	})

	// ============================================================
	// v006: PG clean dirty data
	// ============================================================
	Register(Migration{
		Version:     6,
		Description: "PG: clean dirty algorithm_pick_details data",
		Up: func() error {
			safeExec("UPDATE algorithm_pick_details SET risk_level='', suggestion='' WHERE risk_level IN ('high','medium','low') OR suggestion IN ('buy','hold','sell')")
			return nil
		},
	})

	// ============================================================
	// v007: MySQL GORM AutoMigrate all user/trading/backtest models
	// ============================================================
	Register(Migration{
		Version:     7,
		Description: "MySQL: GORM AutoMigrate user/strategy/backtest models",
		Up: func() error {
			gormAutoMigrate(MySQL,
				&model.User{},
				&model.Watchlist{},
				&model.WatchlistGroup{},
				&model.Strategy{},
				&model.StrategyCondition{},
				&model.BacktestTask{},
				&model.BacktestDailySnapshot{},
				&model.BacktestExecutionLog{},
				&model.BacktestResult{},
				&model.StrategyRun{},
				&model.StrategyComparison{},
				&model.Holding{},
				&model.RiskAlert{},
				&model.ImportLog{},
				&model.CollectionLog{},
				&model.ScheduledTask{},
				&model.TaskLog{},
				&model.LoginLog{},
				&model.AIConfig{},
				&model.Session{},
			)
			return nil
		},
	})

	// ============================================================
	// v008: MySQL fix backtest_results.stock_pool column
	// ============================================================
	Register(Migration{
		Version:     8,
		Description: "MySQL: fix backtest_results.stock_pool column type",
		Up: func() error {
			safeExecMysql("ALTER TABLE backtest_results MODIFY COLUMN stock_pool VARCHAR(30) NOT NULL DEFAULT ''")
			return nil
		},
	})

	log.Printf("[migrate] registered %d migrations", len(migrations))
}
