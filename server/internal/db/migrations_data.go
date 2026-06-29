package db

import (
	"fmt"
	"log"
	"strings"

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
				&model.AlgorithmPick{},model.AlgorithmPick{},
				&model.AlgorithmPick{},model.ConceptAnalysis{},
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

	// ============================================================
	// v009: MySQL PK events / entries / daily rankings
	// ============================================================
	Register(Migration{
		Version:     9,
		Description: "MySQL: pk_events, pk_entries, pk_daily_rankings",
		Up: func() error {
			gormAutoMigrate(MySQL, &model.PkEvent{}, &model.PkEntry{}, &model.PkDailyRanking{})
			return nil
		},
	})


	// ============================================================
	// v010: backtest_signals table
	// ============================================================
	Register(Migration{
		Version:     10,
		Description: "MySQL: backtest_signals for signal-execution decoupling",
		Up: func() error {
			gormAutoMigrate(MySQL, &model.BacktestSignal{})
			return nil
		},
	})


	// ============================================================
	// v011: trading_accounts + trade_records + holding fields
	// ============================================================
	Register(Migration{
		Version:     11,
		Description: "MySQL: trading_accounts, trade_records, holding buy_date/total_cost",
		Up: func() error {
			gormAutoMigrate(MySQL, &model.TradingAccount{}, &model.TradeRecord{})
			// Add columns to holdings — idempotent via information_schema check
			safeExecMysql(`
				SET @col_exists = (SELECT COUNT(*) FROM information_schema.COLUMNS
					WHERE TABLE_SCHEMA = 'stock_predict' AND TABLE_NAME = 'holdings' AND COLUMN_NAME = 'total_cost');
				SET @sql = IF(@col_exists = 0, 'ALTER TABLE holdings ADD COLUMN total_cost DECIMAL(16,2) DEFAULT 0', 'SELECT 1');
				PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
			`)
			safeExecMysql(`
				SET @col_exists = (SELECT COUNT(*) FROM information_schema.COLUMNS
					WHERE TABLE_SCHEMA = 'stock_predict' AND TABLE_NAME = 'holdings' AND COLUMN_NAME = 'buy_date');
				SET @sql = IF(@col_exists = 0, "ALTER TABLE holdings ADD COLUMN buy_date VARCHAR(10) DEFAULT ''", 'SELECT 1');
				PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
			`)
			safeExecMysql(`
				SET @col_exists = (SELECT COUNT(*) FROM information_schema.COLUMNS
					WHERE TABLE_SCHEMA = 'stock_predict' AND TABLE_NAME = 'holdings' AND COLUMN_NAME = 'updated_at');
				SET @sql = IF(@col_exists = 0, 'ALTER TABLE holdings ADD COLUMN updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP', 'SELECT 1');
				PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
			`)
			return nil
		},
	})


	// v006: AI system configs per scene
	migrations = append(migrations, Migration{
		Version:     12,
		Description: "PG: ai_system_configs with default prompts",
		Up: func() error {
			gormAutoMigrate(PG, &model.AISystemConfig{})
			// Insert defaults — idempotent (do nothing if scene already exists)
			defaults := []model.AISystemConfig{
				{
					Scene: "chat_analysis", Name: "AI对话分析",
					SystemPrompt: `你是专业A股分析助手。当前分析标的：__STOCK_CODE__（__STOCK_NAME__），行业：__STOCK_INDUSTRY__。

你拥有以下工具可以实时查询数据库中的精确数据：
- get_stock_price: 获取最新价格、PE/PB、成交量、市值
- get_kline_summary: 获取近期K线走势摘要（均线、涨跌幅）
- get_technical: 获取MACD/KDJ/RSI等技术指标
- get_financials: 获取财务数据（ROE/EPS/营收/利润等）
- get_news: 获取近期新闻和公告
- get_my_holdings: 获取你的持仓数据（成本、数量、盈亏）
- get_shareholders: 获取股东户数和机构持仓比例变化趋势

使用规则：
1. 按问题需求调用工具，只调用回答问题必需的工具。例：问"机构持仓"只需查股东/新闻，不必调技术指标和财务
2. 涉及多只股票时最多深入分析 3 只（选盈亏大或仓位重的），其余简要带过
3. 优先用自然语言回答，贴合用户问题。仅在需要结构化展示（如数据对比、风险清单、操作建议）时使用 Widget
4. 仅在需要结构化展示时使用Widget（JSON格式，w字段必填，每行一个严禁代码块包裹）：
{"w":"summary","label":"短线看多","text":"综合判断≤80字"}
{"w":"signal","u":true,"h":"信号≤10字","d":"说明≤30字"}
{"w":"risk","h":"风险≤10字","d":"说明≤30字"}
{"w":"list","t":"标题≤8字","items":["条目1","条目2","条目3"]}
{"w":"alert","level":"warning","title":"注意","body":"说明"}
{"w":"panel","t":"标题","rows":[{"k":"指标","v":"数值"}]}
{"w":"plan","s":支撑价,"r":压力价,"tip":"建议≤20字","pos":30}
严禁自创格式（如 type/signal 等）或输出<div>等HTML标签，必须使用 w 字段。
5. 分析截止时间：__CURRENT_DATE__`,
					Temperature: 0.7, MaxTokens: 2048, EnableSearch: true,
				},
				{
					Scene: "stock_score", Name: "AI综合评分",
					SystemPrompt: `你是一位资深A股分析师。请全面分析以下股票，从六个维度打分（1-10分），并返回严格JSON格式（不要markdown代码块）：
六维评分标准：
- fundamentalScore(基本面): 营收/利润/ROE/现金流等财务健康度
- growthScore(成长性): 营收增速/利润增速/行业空间  
- valuationScore(估值): PE/PB分位数/与行业对比
- capitalScore(资金面): 成交量/北向资金/主力资金流向
- technicalScore(技术面): 趋势/均线/MACD/KDJ等指标
- industryScore(行业景气): 行业周期/政策/景气度

综合评分cScore 0-10分，建议suggestion(强烈买入/买入/增持/持有/减持/卖出/强烈卖出)，风险等级riskLevel(低风险/中低风险/中风险/中高风险/高风险)，riskWarnings风险点数组，summary 80字以内摘要。__STOCK_DATA__`,
					Temperature: 0.3, MaxTokens: 1024, EnableSearch: false,
				},
			}
			for _, d := range defaults {
				var existing model.AISystemConfig
				if err := PG.Where("scene = ?", d.Scene).First(&existing).Error; err != nil {
					PG.Create(&d)
				}
			}
			return nil
		},
	})


	// v13: composite unique indexes for algorithm_pick_details and predictions
	migrations = append(migrations, Migration{
		Version:     13,
		Description: "PG: composite unique indexes on algorithm_pick_details and predictions",
		Up: func() error {
			gormAutoMigrate(PG, &model.AlgorithmPickDetail{}, &model.Prediction{})
			// Manual unique index creation (GORM doesn't handle composite unique renames well)
			safeExec(`DO $$ BEGIN
				IF NOT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = 'idx_pick_code') THEN
					CREATE UNIQUE INDEX idx_pick_code ON algorithm_pick_details(pick_date, stock_code);
				END IF;
			END $$`)
			safeExec(`DO $$ BEGIN
				IF NOT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = 'idx_pred_code_model_date') THEN
					CREATE UNIQUE INDEX idx_pred_code_model_date ON predictions(code, model_name, predict_date);
				END IF;
			END $$`)
			return nil
		},
	})


	// v14: add enable_tools column to ai_system_configs
	migrations = append(migrations, Migration{
		Version:     14,
		Description: "PG: ai_system_configs add enable_tools column",
		Up: func() error {
			gormAutoMigrate(PG, &model.AISystemConfig{})
			safeExec(`ALTER TABLE ai_system_configs ALTER COLUMN enable_tools SET DEFAULT true`)
			// Update existing rows to have enable_tools = false if null
			safeExec(`UPDATE ai_system_configs SET enable_tools = true WHERE enable_tools IS NULL OR enable_tools = false`)
			return nil
		},
	})


	// v15: add position_sizing column to strategies (idempotent, ignores dup error)
	migrations = append(migrations, Migration{
		Version:     15,
		Description: "MySQL: strategies add position_sizing column",
		Up: func() error {
			if MySQL == nil {
				return nil
			}
			err := MySQL.Exec("ALTER TABLE strategies ADD COLUMN position_sizing VARCHAR(15) DEFAULT 'fixed_pct'").Error
			if err != nil {
				if strings.Contains(err.Error(), "1060") || strings.Contains(err.Error(), "Duplicate") {
					log.Printf("[migrate] v15 column already exists, skipping")
					return nil
				}
				log.Printf("[migrate] v15 WARN: %v", err)
			}
			return nil
		},
	})

	// ============================================================
	// v012: strategy_conditions add enabled column + set existing to true
	// ============================================================
	Register(Migration{
		Version:     12,
		Description: "MySQL: strategy_conditions add enabled column, default true",
		Up: func() error {
			// Add enabled column if not exists (idempotent via GORM AutoMigrate)
			gormAutoMigrate(MySQL, &model.StrategyCondition{})
			// Set existing conditions to enabled=true
			safeExecMysql("UPDATE strategy_conditions SET enabled = true WHERE enabled = false")
			return nil
		},
	})


	// ============================================================
	// v016: stock_profiles table for AI-generated company profiles
	// ============================================================
	Register(Migration{
		Version:     16,
		Description: "PG: stock_profiles table for AI company profiles",
		Up: func() error {
			gormAutoMigrate(PG, &model.StockProfile{})
			return nil
		},
	})


	// v017: stock_realtime_quote table for intraday quote snapshots
	migrations = append(migrations, Migration{
		Version:     17,
		Description: "PG: stock_realtime_quote table for real-time price snapshots",
		Up: func() error {
			gormAutoMigrate(PG, &model.StockRealtimeQuote{})
			return nil
		},
	})


	// v018: stock_profile AI system config for company profile generation
	migrations = append(migrations, Migration{
		Version:     18,
		Description: "PG: stock_profile system config for AI company profiles",
		Up: func() error {
			profile := model.AISystemConfig{
				Scene: "stock_profile", Name: "股票简介",
				SystemPrompt: "你是一位专业、客观、严谨的金融投资分析师，精通A股市场。\n你的任务是对给定的股票进行深度分析，生成一份精美的结构化 Markdown 公司简介。\n\n## 简介结构（严格按此顺序）\n1. **核心特征** — 一句话概括公司定位、盈利模式和当前经营状态\n2. **主营业务** — 业务结构、护城河来源、行业地位\n3. **最新财报** — 表格展示关键财务数据，分析变化原因\n4. **成长驱动** — 短期和长期增长因素\n5. **风险提示** — 3-5条具体风险\n6. **未来展望** — 至少2个前瞻方向\n\n## 格式：Markdown 表格/引用/标题，每部分200字\n\n输出严格JSON：{\"profileMarkdown\":\"...\"}",
				Temperature: 0.7, MaxTokens: 2048, EnableSearch: true,
			}
			var existing model.AISystemConfig
			if err := PG.Where("scene = ?", profile.Scene).First(&existing).Error; err != nil {
				PG.Create(&profile)
			}
			return nil
		},
	})

	
	// v019: agent model config columns for ai_system_configs
	migrations = append(migrations, Migration{
		Version:     19,
		Description: "PG: ai_system_configs add agent model columns",
		Up: func() error {
			gormAutoMigrate(PG, &model.AISystemConfig{})
			return nil
		},
	})

	
	// v020: ai_cost_logs + model_prices for AI cost tracking
	migrations = append(migrations, Migration{
		Version:     20,
		Description: "MySQL: ai_cost_logs and model_prices tables",
		Up: func() error {
			if MySQL == nil {
				return nil
			}
			gormAutoMigrate(MySQL, &model.AICostLog{}, &model.ModelPrice{})

			// Seed default model prices from DeepSeek official pricing
			for _, p := range model.DefaultModelPrices() {
				var existing model.ModelPrice
				if err := MySQL.Where("model_name = ?", p.ModelName).First(&existing).Error; err != nil {
					MySQL.Create(&p)
				}
			}
			return nil
		},
	})

	
	
	// v021: ai_cost_logs add request/response content columns + update model prices
	migrations = append(migrations, Migration{
		Version:     21,
		Description: "MySQL: ai_cost_logs add request_content/response_content + update model_prices",
		Up: func() error {
			if MySQL == nil {
				return nil
			}
			// Auto-migrate new columns
			gormAutoMigrate(MySQL, &model.AICostLog{})
			// Seed/update model prices (V4-Flash and V4-Pro only)
			MySQL.Where("model_name NOT IN ?", []string{"deepseek-v4-flash", "deepseek-v4-pro"}).Delete(&model.ModelPrice{})
			for _, p := range model.DefaultModelPrices() {
				var existing model.ModelPrice
				if err := MySQL.Where("model_name = ?", p.ModelName).First(&existing).Error; err != nil {
					MySQL.Create(&p)
				} else {
					MySQL.Model(&existing).Updates(map[string]interface{}{
						"input_price": p.InputPrice,
						"output_price": p.OutputPrice,
						"cache_hit_price": p.CacheHitPrice,
						"display_name": p.DisplayName,
					})
				}
			}
			return nil
		},
	})

	
	
	// v022: fix stock_score scene name + add strategy_gen/strategy_opt system configs
	migrations = append(migrations, Migration{
		Version:     22,
		Description: "PG: fix stock_score scene rename + add strategy_gen/strategy_opt configs",
		Up: func() error {
			// Rename stock_scoring to stock_score if old name exists
			PG.Exec("UPDATE ai_system_configs SET scene = 'stock_score' WHERE scene = 'stock_scoring'")
			
			// Insert strategy_gen config
			strategyGen := model.AISystemConfig{
				Scene: "strategy_gen", Name: "策略生成",
				SystemPrompt: `你是量化策略专家。请根据用户描述的选股/交易思路，结合下方提供的可用指标参考表，生成一套完整的A股策略条件。

策略要求：
- 策略名称：__STRATEGY_NAME__
- 策略描述：__STRATEGY_DESC__
- 投资风格：__STRATEGY_STYLE__（aggressive=激进/放宽阈值, conservative=保守/收紧阈值, moderate=适中）

请在生成条件前仔细阅读「可用指标参考」表和「条件构建规范」，确保每个条件的 indicator 字段、operator 字段、value 值都严格符合参考表中的定义。`,
				Temperature: 0.7, MaxTokens: 4096, EnableSearch: false,
			}
			var existing model.AISystemConfig
			if err := PG.Where("scene = ?", strategyGen.Scene).First(&existing).Error; err != nil {
				PG.Create(&strategyGen)
			}
			
			// Insert strategy_opt config
			strategyOpt := model.AISystemConfig{
				Scene: "strategy_opt", Name: "策略提示词优化",
				SystemPrompt: `你是一个量化交易策略专家。用户想创建一个A股交易策略，但描述比较简略。请将以下用户要求优化为结构化的策略描述，包含：投资风格、选股偏好、买入时机、卖出时机、仓位管理、风险控制等方面。直接用中文输出优化后的描述，不要加任何前缀说明。

用户原始要求：__USER_PROMPT__
风险偏好：__STRATEGY_STYLE__

优化后的策略描述：`,
				Temperature: 0.7, MaxTokens: 4096, EnableSearch: false,
			}
			if err := PG.Where("scene = ?", strategyOpt.Scene).First(&existing).Error; err != nil {
				PG.Create(&strategyOpt)
			}
			
			return nil
		},
	})

	
	// v023: convert %s placeholders in ai_system_configs to __VAR__ template variables
	migrations = append(migrations, Migration{
		Version:     23,
		Description: "PG: convert %s to __VAR__ in ai_system_configs prompts",
		Up: func() error {
			// chat_analysis: convert old positional %s to named __STOCK_CODE__, etc.
			PG.Exec(`UPDATE ai_system_configs SET system_prompt = REPLACE(system_prompt, '%s（%s），行业：%s', '__STOCK_CODE__（__STOCK_NAME__），行业：__STOCK_INDUSTRY__') WHERE scene = 'chat_analysis'`)
			PG.Exec(`UPDATE ai_system_configs SET system_prompt = REPLACE(system_prompt, '5. 分析截止时间：%s', '5. 分析截止时间：__CURRENT_DATE__') WHERE scene = 'chat_analysis'`)

			// stock_score: convert remaining %s to __STOCK_DATA__ (after summary line)
			PG.Exec(`UPDATE ai_system_configs SET system_prompt = REPLACE(system_prompt, '摘要。%s', '摘要。__STOCK_DATA__') WHERE scene = 'stock_score'`)

			// strategy_gen: convert %s sequence
			PG.Exec(`UPDATE ai_system_configs SET system_prompt = REPLACE(system_prompt, '指标如下：\n%s', '指标如下：\n__INDICATORS__') WHERE scene = 'strategy_gen'`)
			PG.Exec(`UPDATE ai_system_configs SET system_prompt = REPLACE(system_prompt, '策略名称：%s', '策略名称：__STRATEGY_NAME__') WHERE scene = 'strategy_gen'`)
			PG.Exec(`UPDATE ai_system_configs SET system_prompt = REPLACE(system_prompt, '策略描述：%s', '策略描述：__STRATEGY_DESC__') WHERE scene = 'strategy_gen'`)
			PG.Exec(`UPDATE ai_system_configs SET system_prompt = REPLACE(system_prompt, '投资风格：%s', '投资风格：__STRATEGY_STYLE__') WHERE scene = 'strategy_gen'`)

			// strategy_opt: convert %s
			PG.Exec(`UPDATE ai_system_configs SET system_prompt = REPLACE(system_prompt, '用户原始要求：%s', '用户原始要求：__USER_PROMPT__') WHERE scene = 'strategy_opt'`)
			PG.Exec(`UPDATE ai_system_configs SET system_prompt = REPLACE(system_prompt, '风险偏好：%s', '风险偏好：__STRATEGY_STYLE__') WHERE scene = 'strategy_opt'`)

			return nil
		},
	})



	// v024: market sentiment tables + board_type/is_st on stocks_basic
	migrations = append(migrations, Migration{
		Version:     24,
		Description: "PG: board_type/is_st on stocks_basic + market_sentiment/northbound_flow/stock_capital_flow/sentiment_weights",
		Up: func() error {
			// ============================================================
			// 1. Add board_type and is_st to stocks_basic
			// ============================================================
			safeExec(`ALTER TABLE stocks_basic ADD COLUMN IF NOT EXISTS board_type VARCHAR(5)`)
			safeExec(`ALTER TABLE stocks_basic ADD COLUMN IF NOT EXISTS is_st BOOLEAN DEFAULT FALSE`)

			// Backfill board_type from code prefix
			safeExec(`UPDATE stocks_basic SET board_type = 'sh' WHERE code LIKE '60%' AND board_type IS NULL`)
			safeExec(`UPDATE stocks_basic SET board_type = 'kc' WHERE code LIKE '68%' AND board_type IS NULL`)
			safeExec(`UPDATE stocks_basic SET board_type = 'sz' WHERE code LIKE '00%' AND board_type IS NULL`)
			safeExec(`UPDATE stocks_basic SET board_type = 'cy' WHERE code LIKE '30%' AND board_type IS NULL`)
			safeExec(`UPDATE stocks_basic SET board_type = 'bj' WHERE code ~ '^[89]' AND board_type IS NULL`)

			// Backfill is_st from name
			safeExec(`UPDATE stocks_basic SET is_st = TRUE WHERE (name LIKE '%ST%' OR name LIKE '%*ST%') AND NOT is_st`)

			// ============================================================
			// 2. market_sentiment — daily composite sentiment
			// ============================================================
			safeExec(`CREATE TABLE IF NOT EXISTS market_sentiment (
				trade_date DATE PRIMARY KEY,
				market_breadth NUMERIC(6,4),
				breadth_score NUMERIC(5,2),
				style_risk_pref NUMERIC(6,4),
				style_risk_score NUMERIC(5,2),
				trade_activity NUMERIC(6,4),
				activity_score NUMERIC(5,2),
				profit_effect NUMERIC(6,4),
				profit_score NUMERIC(5,2),
				volatility NUMERIC(6,4),
				vol_score NUMERIC(5,2),
				price_strength NUMERIC(6,4),
				strength_score NUMERIC(5,2),
				risk_appetite NUMERIC(6,4),
				risk_app_score NUMERIC(5,2),
				limit_sentiment NUMERIC(6,4),
				limit_score NUMERIC(5,2),
				sector_diffusion NUMERIC(6,4),
				sector_score NUMERIC(5,2),
				northbound_net NUMERIC(12,2),
				northbound_score NUMERIC(5,2),
				capital_flow_net NUMERIC(12,2),
				capital_flow_score NUMERIC(5,2),
				composite_score NUMERIC(5,2),
				up_count INT DEFAULT 0,
				down_count INT DEFAULT 0,
				limit_up_count INT DEFAULT 0,
				limit_down_count INT DEFAULT 0,
				board_break_count INT DEFAULT 0,
				total_stocks INT DEFAULT 0,
				created_at TIMESTAMPTZ DEFAULT NOW()
			)`)
			safeExec(`CREATE INDEX IF NOT EXISTS idx_market_sentiment_date ON market_sentiment(trade_date DESC)`)

			// ============================================================
			// 3. northbound_flow — daily northbound capital flow
			// ============================================================
			safeExec(`CREATE TABLE IF NOT EXISTS northbound_flow (
				trade_date DATE PRIMARY KEY,
				hgt_net NUMERIC(12,2),
				sgt_net NUMERIC(12,2),
				total_net NUMERIC(12,2),
				hgt_balance NUMERIC(12,2),
				sgt_balance NUMERIC(12,2),
				created_at TIMESTAMPTZ DEFAULT NOW()
			)`)
			safeExec(`CREATE INDEX IF NOT EXISTS idx_northbound_date ON northbound_flow(trade_date DESC)`)

			// ============================================================
			// 4. stock_capital_flow — per-stock daily capital flow
			// ============================================================
			safeExec(`CREATE TABLE IF NOT EXISTS stock_capital_flow (
				code VARCHAR(10),
				trade_date DATE,
				main_net NUMERIC(16,2),
				super_large_net NUMERIC(16,2),
				large_net NUMERIC(16,2),
				medium_net NUMERIC(16,2),
				small_net NUMERIC(16,2),
				PRIMARY KEY (code, trade_date)
			)`)
			safeExec(`CREATE INDEX IF NOT EXISTS idx_capital_flow_code ON stock_capital_flow(code)`)
			safeExec(`CREATE INDEX IF NOT EXISTS idx_capital_flow_date ON stock_capital_flow(trade_date DESC)`)

			// ============================================================
			// 5. sentiment_weights — configurable weight schemes
			// ============================================================
			safeExec(`CREATE TABLE IF NOT EXISTS sentiment_weights (
				id SERIAL PRIMARY KEY,
				name VARCHAR(50) NOT NULL,
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
			)`)

			// Seed default equal-weight scheme
			safeExec(`INSERT INTO sentiment_weights (name, is_active)
				SELECT '等权默认', TRUE
				WHERE NOT EXISTS (SELECT 1 FROM sentiment_weights WHERE is_active = TRUE)`)

			// ============================================================
			// 6. GORM AutoMigrate for new models
			// ============================================================
			gormAutoMigrate(PG,
				&model.MarketSentiment{},
				&model.MarketSentiment{},
				&model.NorthboundMinute{},
				&model.StockCapitalFlow{},
				&model.SentimentWeights{},
				&model.StockCapitalFlow{},
				&model.SentimentWeights{},
			)

			// ============================================================
			// 7. Create northbound_daily_view
			// ============================================================
			safeExec(`CREATE OR REPLACE VIEW northbound_daily_view AS
				SELECT trade_date,
					MAX(hgt_cumulative) - MIN(hgt_cumulative) AS hgt_net,
					MAX(sgt_cumulative) - MIN(sgt_cumulative) AS sgt_net,
					MAX(hgt_cumulative) - MIN(hgt_cumulative) + MAX(sgt_cumulative) - MIN(sgt_cumulative) AS total_net
				FROM northbound_minute
				GROUP BY trade_date`)

			return nil
		},
	})



	// v025: ETF board_type backfill
	Register(Migration{
		Version:     25,
		Description: "PG: backfill ETF/bond board_type on stocks_basic",
		Up: func() error {
			// Bond ETFs (国债ETF)
			safeExec(`UPDATE stocks_basic SET board_type = 'bond' WHERE code LIKE '511%' AND board_type IS NULL`)
			safeExec(`UPDATE stocks_basic SET board_type = 'bond' WHERE code LIKE '1596%' AND board_type IS NULL`)
			// General ETFs (code not matching sh/sz/kc/cy/bj/bond patterns and not IDX)
			safeExec(`UPDATE stocks_basic SET board_type = 'etf' WHERE code LIKE '51%' AND board_type IS NULL AND code !~ '^IDX'`)
			safeExec(`UPDATE stocks_basic SET board_type = 'etf' WHERE code LIKE '159%' AND board_type IS NULL AND code !~ '^IDX'`)
			safeExec(`UPDATE stocks_basic SET board_type = 'etf' WHERE code LIKE '56%' AND board_type IS NULL AND code !~ '^IDX'`)
			safeExec(`UPDATE stocks_basic SET board_type = 'etf' WHERE code LIKE '58%' AND board_type IS NULL AND code !~ '^IDX'`)
			return nil
		},
	})

	// v026: Strategy orchestration v2 — weighted scoring, decision tree, AI agent
	Register(Migration{
		Version:     26,
		Description: "PG+MySQL: strategy orchestration v2 — scoring/decision_tree/hybrid, condition weight/fuzzy/trend/industry/timeframe/composite, AI agent decisions",
		Up: func() error {
			// ── PostgreSQL: ai_agent_decisions ──
			safeExec(`CREATE TABLE IF NOT EXISTS ai_agent_decisions (
				id SERIAL PRIMARY KEY,
				strategy_id INTEGER NOT NULL,
				backtest_task_id INTEGER,
				trade_date VARCHAR(10) NOT NULL,
				market_score NUMERIC(5,2) DEFAULT 0,
				market_bias NUMERIC(5,2) DEFAULT 1.0,
				candidates_in INTEGER DEFAULT 0,
				candidates_out INTEGER DEFAULT 0,
				reasoning TEXT DEFAULT \'\',
				actions JSONB DEFAULT \'[]\',
				overrides_applied BOOLEAN DEFAULT FALSE,
				created_at TIMESTAMPTZ DEFAULT NOW()
			)`)
			safeExec(`CREATE INDEX IF NOT EXISTS idx_ai_agent_decisions_strategy ON ai_agent_decisions(strategy_id)`)
			safeExec(`CREATE INDEX IF NOT EXISTS idx_ai_agent_decisions_task ON ai_agent_decisions(backtest_task_id)`)

			// ── MySQL: AutoMigrate adds new columns on strategies and strategy_conditions ──
			gormAutoMigrate(MySQL,
				&model.Strategy{},
				&model.StrategyCondition{},
				&model.ConditionTemplate{},
			)

			return nil
		},
	})


	// v027: Add Policy Manager v3 fields to MySQL strategies table
	Register(Migration{
		Version:     27,
		Description: "MySQL: strategies add policy_mode, aggressive_threshold, defensive_threshold, policy_aggressive, policy_defensive, policy_cash columns",
		Up: func() error {
			gormAutoMigrate(MySQL, &model.Strategy{})
			return nil
		},
	})

	// v028: MACD precomputed columns on stocks_daily_k for real EMA-based computation
	Register(Migration{
		Version:     28,
		Description: "PG: stocks_daily_k add ema12, ema26, macd_dif, macd_dea precomputed columns",
		Up: func() error {
			for _, col := range []struct{ name, typ string }{
				{"ema12", "NUMERIC(12,4)"},
				{"ema26", "NUMERIC(12,4)"},
				{"macd_dif", "NUMERIC(12,4)"},
				{"macd_dea", "NUMERIC(12,4)"},
			} {
				if err := PG.Exec(fmt.Sprintf("ALTER TABLE stocks_daily_k ADD COLUMN IF NOT EXISTS %s %s", col.name, col.typ)).Error; err != nil {
					log.Printf("[migrate:v028] add column %s: %v", col.name, err)
				}
			}
			// Create index on the new columns for fast lookups
			for _, col := range []string{"ema12", "ema26", "macd_dif", "macd_dea"} {
				idxName := fmt.Sprintf("idx_stocks_daily_k_%s", col)
				PG.Exec(fmt.Sprintf("CREATE INDEX IF NOT EXISTS %s ON stocks_daily_k(code, trade_date, %s)", idxName, col))
			}
			return nil
		},
	})

	// v029: Strategy risk limit fields
	Register(Migration{
		Version:     29,
		Description: "MySQL: strategies add position_concentration_limit, max_daily_loss columns",
		Up: func() error {
			gormAutoMigrate(MySQL, &model.Strategy{})
			return nil
		},
	})

	// v030: Strategy soft-delete support
	Register(Migration{
		Version:     30,
		Description: "MySQL: strategies add deleted_at column for soft-delete",
		Up: func() error {
			gormAutoMigrate(MySQL, &model.Strategy{})
			return nil
		},
	})

// v031: Trailing stop columns on strategies
Register(Migration{
    Version:     31,
    Description: "MySQL: strategies add enable_trailing_stop, trailing_stop_activation, trailing_stop_drawdown columns",
    Up: func() error {
        gormAutoMigrate(MySQL, &model.Strategy{})
        return nil
    },
})

// v032: Dip buy columns on strategies
Register(Migration{
    Version:     32,
    Description: "MySQL: strategies add enable_dip_buy, dip_buy_threshold, dip_buy_amount_pct, dip_target_return, dip_max_hold_days, dip_cooldown_days columns",
    Up: func() error {
        gormAutoMigrate(MySQL, &model.Strategy{})
        return nil
    },
})

// v033: Grid trading columns on strategies
Register(Migration{
    Version:     33,
    Description: "MySQL: strategies add enable_grid, grid_trigger_squeeze, grid_levels, grid_lot_pct columns",
    Up: func() error {
        gormAutoMigrate(MySQL, &model.Strategy{})
        return nil
    },
})


// v034: market_style_daily table for daily market style classification and review
Register(Migration{
    Version:     34,
    Description: "PG: market_style_daily table with style classification, structural analysis, and review data",
    Up: func() error {
        safeExec(`CREATE TABLE IF NOT EXISTS market_style_daily (
            trade_date        DATE PRIMARY KEY,
            style             VARCHAR(20) NOT NULL,
            style_confidence  NUMERIC(5,2) DEFAULT 0,
            composite_score   NUMERIC(5,2),
            up_ratio          NUMERIC(5,4),
            sector_diffusion  NUMERIC(5,4),
            volatility        NUMERIC(5,4),
            score_trend       NUMERIC(7,4),
            northbound_net    NUMERIC(12,2),
            total_amount      NUMERIC(20,2),
            limit_up_count    INT,
            limit_down_count  INT,
            ma20_above        INT,
            n52_high          INT,
            n60_low           INT,
            style_duration    INT DEFAULT 0,
            transition_signal VARCHAR(20),
            top_sectors       JSONB,
            top_concepts      JSONB,
            analysis_summary  TEXT,
            created_at        TIMESTAMPTZ DEFAULT now()
        )`)
        safeExec(`CREATE INDEX IF NOT EXISTS idx_market_style_date ON market_style_daily(trade_date)`)
        safeExec(`CREATE INDEX IF NOT EXISTS idx_market_style_name ON market_style_daily(style)`)
        return nil
    },
})


	// v035: concept_analyses table for AI-generated concept board analysis
	Register(Migration{
		Version: 35,
		Description: "PG: concept_analyses table for AI concept board analysis cache",
		Up: func() error {
			safeExec(`CREATE TABLE IF NOT EXISTS concept_analyses (
				id SERIAL PRIMARY KEY,
				concept_code VARCHAR(20) UNIQUE NOT NULL,
				content TEXT,
				generated_at TIMESTAMPTZ,
				created_at TIMESTAMPTZ DEFAULT now(),
				updated_at TIMESTAMPTZ DEFAULT now()
			)`)
			safeExec(`CREATE INDEX IF NOT EXISTS idx_concept_analyses_code ON concept_analyses(concept_code)`)
			return nil
		},
	})

	// v036: seed concept_analysis system config
	Register(Migration{
		Version:     36,
		Description: "PG: seed concept_analysis ai_system_configs entry",
		Up: func() error {
			var existing model.AISystemConfig
			if PG.Where("scene = ?", "concept_analysis").First(&existing).Error != nil {
				PG.Create(&model.AISystemConfig{
					Scene:        "concept_analysis",
					Name:         "概念分析",
					SystemPrompt: "你是一位资深证券分析师，请对以下概念板块进行全面分析。\n\n## 要求\n1. **概念概述**：用一段话简要介绍该概念的核心定义、行业背景\n2. **龙头股票**：列出3-5只核心龙头股（代码+名称+简要逻辑）\n3. **商业模式**：分析该概念的典型商业模式和盈利模式\n4. **利润拆分**：拆解产业链各环节的利润分配（上游/中游/下游）\n5. **上下游产业链**：详细分析上游供应商、中游制造/服务、下游应用\n6. **投资逻辑**：核心投资逻辑和关键跟踪指标\n7. **风险提示**：行业面临的主要风险\n\n请使用专业的Markdown格式输出，适当使用表格和列表，语言精炼专业。",
					Temperature: 0.7, MaxTokens: 4096, EnableSearch: false,
				})
			}
			return nil
		},
	})

	// v037: dragon_tiger_list + dragon_tiger_detail + margin_trading
	Register(Migration{
		Version:     37,
		Description: "PG: dragon_tiger_list, dragon_tiger_detail, margin_trading tables",
		Up: func() error {
			safeExec(`CREATE TABLE IF NOT EXISTS dragon_tiger_list (
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
			)`)
			safeExec(`CREATE INDEX IF NOT EXISTS idx_dragon_tiger_list_code ON dragon_tiger_list(code)`)
			safeExec(`CREATE INDEX IF NOT EXISTS idx_dragon_tiger_list_date ON dragon_tiger_list(trade_date)`)
			safeExec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_dragon_tiger_list_code_date ON dragon_tiger_list(code, trade_date)`)
			safeExec(`CREATE TABLE IF NOT EXISTS dragon_tiger_detail (
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
			)`)
			safeExec(`CREATE INDEX IF NOT EXISTS idx_dragon_tiger_detail_code ON dragon_tiger_detail(code)`)
			safeExec(`CREATE INDEX IF NOT EXISTS idx_dragon_tiger_detail_date ON dragon_tiger_detail(trade_date)`)
			safeExec(`CREATE TABLE IF NOT EXISTS margin_trading (
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
			)`)
			safeExec(`CREATE INDEX IF NOT EXISTS idx_margin_trading_code ON margin_trading(code)`)
			safeExec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_margin_trading_code_date ON margin_trading(code, trade_date)`)
			return nil
		},
	})

	// v038: block_trade + restricted_share_unlock
	Register(Migration{
		Version:     38,
		Description: "PG: block_trade, restricted_share_unlock tables",
		Up: func() error {
			safeExec(`CREATE TABLE IF NOT EXISTS block_trade (
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
			)`)
			safeExec(`CREATE INDEX IF NOT EXISTS idx_block_trade_code ON block_trade(code)`)
			safeExec(`CREATE INDEX IF NOT EXISTS idx_block_trade_date ON block_trade(trade_date)`)
			safeExec(`CREATE TABLE IF NOT EXISTS restricted_share_unlock (
				id SERIAL PRIMARY KEY,
				code VARCHAR(10),
				free_date DATE,
				stock_type VARCHAR(100),
				shares NUMERIC(24,2),
				ratio NUMERIC(12,4),
				is_history BOOLEAN DEFAULT true,
				created_at TIMESTAMPTZ DEFAULT now()
			)`)
			safeExec(`CREATE INDEX IF NOT EXISTS idx_restricted_unlock_code ON restricted_share_unlock(code)`)
			safeExec(`CREATE INDEX IF NOT EXISTS idx_restricted_unlock_date ON restricted_share_unlock(free_date)`)
			safeExec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_restricted_unlock_unique ON restricted_share_unlock(code, free_date, stock_type)`)
			return nil
		},
	})

	// v039: ths_hot_stocks + dividend_history
	Register(Migration{
		Version:     39,
		Description: "PG: ths_hot_stocks, dividend_history tables",
		Up: func() error {
			safeExec(`CREATE TABLE IF NOT EXISTS ths_hot_stocks (
				id SERIAL PRIMARY KEY,
				code VARCHAR(10),
				name VARCHAR(50),
				trade_date DATE,
				close_price NUMERIC(12,4),
				change_amount NUMERIC(12,4),
				change_pct NUMERIC(8,4),
				turnover_pct NUMERIC(8,4),
				volume NUMERIC(24,2),
				amount NUMERIC(24,2),
				dde_net_amount NUMERIC(16,2),
				reason_tags TEXT,
				market VARCHAR(5),
				created_at TIMESTAMPTZ DEFAULT now()
			)`)
			safeExec(`CREATE INDEX IF NOT EXISTS idx_ths_hot_date ON ths_hot_stocks(trade_date)`)
			safeExec(`CREATE INDEX IF NOT EXISTS idx_ths_hot_code ON ths_hot_stocks(code)`)
			safeExec(`CREATE TABLE IF NOT EXISTS dividend_history (
				id SERIAL PRIMARY KEY,
				code VARCHAR(10),
				ex_dividend_date DATE,
				bonus_rmb NUMERIC(10,4),
				transfer_ratio NUMERIC(10,4),
				bonus_ratio NUMERIC(10,4),
				progress VARCHAR(50),
				created_at TIMESTAMPTZ DEFAULT now()
			)`)
			safeExec(`CREATE INDEX IF NOT EXISTS idx_dividend_code ON dividend_history(code)`)
			safeExec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_dividend_code_date ON dividend_history(code, ex_dividend_date)`)
			return nil
		},
	})

	// v040: ths_eps_forecast + cninfo_announcements + macro_news
	Register(Migration{
		Version:     40,
		Description: "PG: ths_eps_forecast, cninfo_announcements, macro_news tables",
		Up: func() error {
			safeExec(`CREATE TABLE IF NOT EXISTS ths_eps_forecast (
				id SERIAL PRIMARY KEY,
				code VARCHAR(10),
				year VARCHAR(10),
				institution_count INT DEFAULT 0,
				eps_min NUMERIC(12,6),
				eps_avg NUMERIC(12,6),
				eps_max NUMERIC(12,6),
				created_at TIMESTAMPTZ DEFAULT now()
			)`)
			safeExec(`CREATE INDEX IF NOT EXISTS idx_ths_eps_code ON ths_eps_forecast(code)`)
			safeExec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_ths_eps_code_year ON ths_eps_forecast(code, year)`)
			safeExec(`CREATE TABLE IF NOT EXISTS cninfo_announcements (
				id SERIAL PRIMARY KEY,
				code VARCHAR(10),
				title VARCHAR(500),
				ann_type VARCHAR(50),
				ann_date DATE,
				ann_url VARCHAR(500),
				created_at TIMESTAMPTZ DEFAULT now()
			)`)
			safeExec(`CREATE INDEX IF NOT EXISTS idx_cninfo_ann_code ON cninfo_announcements(code)`)
			safeExec(`CREATE INDEX IF NOT EXISTS idx_cninfo_ann_date ON cninfo_announcements(ann_date)`)
			safeExec(`CREATE TABLE IF NOT EXISTS macro_news (
				id SERIAL PRIMARY KEY,
				title VARCHAR(500),
				summary TEXT,
				news_time VARCHAR(30),
				category VARCHAR(30) DEFAULT 'general',
				created_at TIMESTAMPTZ DEFAULT now()
			)`)
			safeExec(`CREATE INDEX IF NOT EXISTS idx_macro_news_time ON macro_news(news_time)`)
			safeExec(`CREATE INDEX IF NOT EXISTS idx_macro_news_category ON macro_news(category)`)
			return nil
		},
	})

	Register(Migration{
		Version:     41,
		Description: "MySQL: add behavior_stats column to collection_logs",
		Up: func() error {
			gormAutoMigrate(MySQL, &model.CollectionLog{})
			return nil
		},
	})

	log.Printf("[migrate] registered %d migrations", len(migrations))



}
