package db

import (
	"log"

	"github.com/ai-stock-predict/server/internal/model"
)

func AutoMigrate() {
	if PG != nil {
		PG.Exec(`CREATE TABLE IF NOT EXISTS stock_signals (
			id SERIAL PRIMARY KEY,
			code VARCHAR(10) UNIQUE,
			signal_value NUMERIC(12,6),
			source VARCHAR(50) DEFAULT 'excel_import',
			updated_at TIMESTAMPTZ DEFAULT NOW()
		)`)

		if err := PG.AutoMigrate(
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
			&model.PredictionKDist{},
		); err != nil {
			log.Printf("PG migrate warning: %v", err)
		}
	}

	if MySQL != nil {
		if err := MySQL.AutoMigrate(
			&model.User{},
			&model.Watchlist{},
			&model.WatchlistGroup{},
			&model.Strategy{},
			&model.StrategyCondition{},
			&model.BacktestResult{},model.BacktestResult{},
			&model.BacktestResult{},model.BacktestTask{},
			&model.Holding{},
			&model.RiskAlert{},
			&model.ImportLog{},
			&model.CollectionLog{},
			&model.ScheduledTask{},
			&model.TaskLog{},
			&model.LoginLog{},
			&model.AIConfig{},
			&model.Session{},
		); err != nil {
			log.Printf("MySQL migrate warning: %v", err)
		}
	}
	// Clean dirty historical data: old English risk/suggestion -> empty
	if PG != nil {
		PG.Exec("UPDATE algorithm_pick_details SET risk_level='', suggestion='' WHERE risk_level IN ('high','medium','low') OR suggestion IN ('buy','hold','sell')")
	}
	log.Println("AutoMigrate completed")
}

// EnsureManualTables creates tables that GORM AutoMigrate might miss
func EnsureManualTables() {
	if PG != nil {
		PG.Exec(`CREATE TABLE IF NOT EXISTS ai_conversations (
			id SERIAL PRIMARY KEY,
			code VARCHAR(10),
			role VARCHAR(10),
			content TEXT,
			created_at TIMESTAMPTZ DEFAULT NOW()
		)`)
		PG.Exec(`CREATE INDEX IF NOT EXISTS idx_ai_conv_code ON ai_conversations(code)`)
		PG.Exec(`CREATE TABLE IF NOT EXISTS predictions (
			id SERIAL PRIMARY KEY,
			code VARCHAR(10),
			model_name VARCHAR(30),
			predict_date DATE,
			predicted_price NUMERIC(12,4),
			upper_bound NUMERIC(12,4),
			lower_bound NUMERIC(12,4),
			created_at TIMESTAMPTZ DEFAULT NOW()
		)`)
		PG.Exec(`CREATE INDEX IF NOT EXISTS idx_pred_code ON predictions(code)`)
		PG.Exec(`CREATE TABLE IF NOT EXISTS ai_analyses (
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
		PG.Exec(`CREATE INDEX IF NOT EXISTS idx_ai_analysis_code ON ai_analyses(code)`)
		PG.Exec(`CREATE TABLE IF NOT EXISTS ai_stock_scores (
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
		PG.Exec(`CREATE INDEX IF NOT EXISTS idx_ai_score_code ON ai_stock_scores(code)`)
		PG.Exec(`CREATE TABLE IF NOT EXISTS stock_shareholders (
			id SERIAL PRIMARY KEY, code VARCHAR(10), report_date VARCHAR(10),
			total_holders BIGINT, holder_change NUMERIC(10,4),
			top10_holders JSONB, top10_float JSONB,
			inst_hold_ratio NUMERIC(10,4), avg_holding BIGINT,
			created_at TIMESTAMPTZ DEFAULT NOW()
		)`)
		PG.Exec(`DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='uq_shareholders') THEN ALTER TABLE stock_shareholders ADD CONSTRAINT uq_shareholders UNIQUE (code, report_date); END IF; END $$`)
		PG.Exec(`CREATE TABLE IF NOT EXISTS stock_financials (
			id SERIAL PRIMARY KEY, code VARCHAR(10), report_date VARCHAR(10),
			report_type VARCHAR(10), total_revenue NUMERIC(20,2), net_profit NUMERIC(20,2),
			revenue_growth NUMERIC(10,4), profit_growth NUMERIC(10,4),
			total_assets NUMERIC(20,2), total_liabilities NUMERIC(20,2), net_assets NUMERIC(20,2),
			roe NUMERIC(10,4), eps NUMERIC(10,4), bps NUMERIC(10,4),
			gross_margin NUMERIC(10,4), net_margin NUMERIC(10,4), debt_ratio NUMERIC(10,4),
			created_at TIMESTAMPTZ DEFAULT NOW()
		)`)
		PG.Exec(`DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='uq_financials') THEN ALTER TABLE stock_financials ADD CONSTRAINT uq_financials UNIQUE (code, report_date); END IF; END $$`)
		PG.Exec(`CREATE TABLE IF NOT EXISTS stock_news (
			id SERIAL PRIMARY KEY, code VARCHAR(10), title VARCHAR(500),
			summary TEXT, source VARCHAR(50), news_type VARCHAR(20),
			url VARCHAR(500), publish_date VARCHAR(10),
			created_at TIMESTAMPTZ DEFAULT NOW()
		)`)
		PG.Exec(`CREATE INDEX IF NOT EXISTS idx_news_code ON stock_news(code)`)
		PG.Exec(`DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='uq_news') THEN ALTER TABLE stock_news ADD CONSTRAINT uq_news UNIQUE (code, title, publish_date); END IF; END $$`)

		PG.Exec(`CREATE TABLE IF NOT EXISTS stock_reports (
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
		PG.Exec(`CREATE INDEX IF NOT EXISTS idx_reports_code ON stock_reports(stock_code)`)
		PG.Exec(`CREATE INDEX IF NOT EXISTS idx_reports_date ON stock_reports(publish_date)`)
		PG.Exec(`CREATE INDEX IF NOT EXISTS idx_reports_industry ON stock_reports(industry_name)`)

		PG.Exec(`CREATE TABLE IF NOT EXISTS prediction_kdist (
			id SERIAL PRIMARY KEY,
			code VARCHAR(10) UNIQUE,
			kd_data JSONB,
			updated_at TIMESTAMPTZ DEFAULT NOW()
		)`)
	}
}
