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


	Register(Migration{
		Version:     42,
		Description: "PG: stock_fund_flow table — daily main/small/mid/large/super net flow",
		Up: func() error {
			safeExec(`CREATE TABLE IF NOT EXISTS stock_fund_flow (
				id SERIAL PRIMARY KEY,
				code VARCHAR(10),
				trade_date DATE,
				main_net NUMERIC(18,4) DEFAULT 0,
				small_net NUMERIC(18,4) DEFAULT 0,
				mid_net NUMERIC(18,4) DEFAULT 0,
				large_net NUMERIC(18,4) DEFAULT 0,
				super_net NUMERIC(18,4) DEFAULT 0,
				created_at TIMESTAMPTZ DEFAULT now()
			)`)
			safeExec(`CREATE INDEX IF NOT EXISTS idx_stock_fund_flow_code ON stock_fund_flow(code)`)
			safeExec(`CREATE INDEX IF NOT EXISTS idx_stock_fund_flow_date ON stock_fund_flow(trade_date)`)
			safeExec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_stock_fund_flow_unique ON stock_fund_flow(code, trade_date)`)
			return nil
		},
	})


	Register(Migration{
		Version:     43,
		Description: "PG: add unique indexes for ON CONFLICT support on collector tables",
		Up: func() error {
			safeExec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_dragon_tiger_list_unique ON dragon_tiger_list (code, trade_date)`)
			safeExec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_dragon_tiger_detail_unique ON dragon_tiger_detail (code, trade_date, seat_name, net_amt)`)
			safeExec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_margin_trading_unique ON margin_trading (code, trade_date)`)
			safeExec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_block_trade_unique ON block_trade (code, trade_date, deal_price, deal_volume)`)
			safeExec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_cninfo_announcements_unique ON cninfo_announcements (code, title, ann_date)`)
			safeExec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_macro_news_unique ON macro_news (title, news_time)`)
			safeExec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_dividend_history_unique ON dividend_history (code, ex_dividend_date)`)
			safeExec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_ths_hot_stocks_unique ON ths_hot_stocks (code, trade_date)`)
			safeExec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_ths_eps_forecast_unique ON ths_eps_forecast (code, year)`)
			safeExec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_restricted_unlock_unique ON restricted_share_unlock (code, free_date, stock_type)`)
			return nil
		},
	})

	log.Printf("[migrate] registered %d migrations", len(migrations))




	// ============================================================
	// v044: MySQL seed missing scheduled_tasks rows
	// ============================================================
	Register(Migration{
		Version:     44,
		Description: "MySQL: seed missing scheduled_tasks (fund_flow)",
		Up: func() error {
			seeds := []struct{ Name, Phase, CronExpr string }{
				{"资金流向采集", "fund_flow", "0 0 17 * * 1-5"},
			}
			for _, s := range seeds {
				var count int64
				MySQL.Raw("SELECT COUNT(*) FROM scheduled_tasks WHERE phase = ?", s.Phase).Scan(&count)
				if count == 0 {
					MySQL.Exec("INSERT INTO scheduled_tasks (name, phase, cron_expr, enabled, created_at, updated_at) VALUES (?, ?, ?, true, NOW(), NOW())",
						s.Name, s.Phase, s.CronExpr)
					log.Printf("[migrate v44] seeded scheduled_task: %s", s.Phase)
				}
			}
			return nil
		},
	})

	// v045: stocks_daily_k extended columns for Youzi K-line API
	Register(Migration{
		Version:     45,
		Description: "PG: stocks_daily_k add data_source + high_limit + low_limit + avg_price + is_paused columns",
		Up: func() error {
			safeExec(`ALTER TABLE stocks_daily_k ADD COLUMN IF NOT EXISTS data_source VARCHAR(20) DEFAULT 'tencent'`)
			safeExec(`ALTER TABLE stocks_daily_k ADD COLUMN IF NOT EXISTS high_limit NUMERIC`)
			safeExec(`ALTER TABLE stocks_daily_k ADD COLUMN IF NOT EXISTS low_limit NUMERIC`)
			safeExec(`ALTER TABLE stocks_daily_k ADD COLUMN IF NOT EXISTS avg_price NUMERIC`)
			safeExec(`ALTER TABLE stocks_daily_k ADD COLUMN IF NOT EXISTS is_paused BOOLEAN DEFAULT false`)
			return nil
		},
	})
	// v046: limit_stats_daily pre-computed table for fast limit-up/down dashboard
	Register(Migration{
		Version:     46,
		Description: "PG: create limit_stats_daily for pre-computed limit-up/down stats",
		Up: func() error {
			safeExec(`CREATE TABLE IF NOT EXISTS limit_stats_daily (
				id SERIAL PRIMARY KEY,
				trade_date DATE NOT NULL,
				up_count INT DEFAULT 0,
				down_count INT DEFAULT 0,
				rise_count INT DEFAULT 0,
				fall_count INT DEFAULT 0,
				board_break INT DEFAULT 0,
				total_stocks INT DEFAULT 0,
				created_at TIMESTAMPTZ DEFAULT now()
			)`)
			safeExec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_limit_stats_daily_date ON limit_stats_daily(trade_date)`)
			return nil
		},
	})

	// v047: stocks_daily_k daily-frequency fields from Tencent qt
	Register(Migration{
		Version:     47,
		Description: "PG: add buy_vol/sell_vol/change_pct/amplitude/volume_ratio to stocks_daily_k, populate high_limit/low_limit/avg_price from qt",
		Up: func() error {
			safeExec(`ALTER TABLE stocks_daily_k ADD COLUMN IF NOT EXISTS buy_vol BIGINT DEFAULT 0`)
			safeExec(`ALTER TABLE stocks_daily_k ADD COLUMN IF NOT EXISTS sell_vol BIGINT DEFAULT 0`)
			safeExec(`ALTER TABLE stocks_daily_k ADD COLUMN IF NOT EXISTS change_pct NUMERIC(8,4) DEFAULT 0`)
			safeExec(`ALTER TABLE stocks_daily_k ADD COLUMN IF NOT EXISTS amplitude NUMERIC(8,4) DEFAULT 0`)
			safeExec(`ALTER TABLE stocks_daily_k ADD COLUMN IF NOT EXISTS volume_ratio NUMERIC(8,4) DEFAULT 0`)
			// amount was previously computed as close×volume; keep but prefer qt[37] 成交额(万)
			// high_limit/low_limit/avg_price columns already exist from v045, will be populated by batch_collect
			return nil
		},
	})


	// v048: Tushare daily fields: pre_close + change_amount
	Register(Migration{
		Version:     48,
		Description: "PG: add pre_close and change_amount to stocks_daily_k for Tushare data source",
		Up: func() error {
			safeExec(`ALTER TABLE stocks_daily_k ADD COLUMN IF NOT EXISTS pre_close NUMERIC(12,4) DEFAULT 0`)
			safeExec(`ALTER TABLE stocks_daily_k ADD COLUMN IF NOT EXISTS change_amount NUMERIC(12,4) DEFAULT 0`)
			return nil
		},
	})


	// v049: Tushare daily_basic indicator fields
	Register(Migration{
		Version:     49,
		Description: "PG: add daily_basic indicator fields to stocks_daily_indicator",
		Up: func() error {
			safeExec(`ALTER TABLE stocks_daily_indicator ADD COLUMN IF NOT EXISTS pe_ttm NUMERIC(14,4) DEFAULT 0`)
			safeExec(`ALTER TABLE stocks_daily_indicator ADD COLUMN IF NOT EXISTS ps_ttm NUMERIC(14,4) DEFAULT 0`)
			safeExec(`ALTER TABLE stocks_daily_indicator ADD COLUMN IF NOT EXISTS turnover_rate NUMERIC(10,4) DEFAULT 0`)
			safeExec(`ALTER TABLE stocks_daily_indicator ADD COLUMN IF NOT EXISTS turnover_rate_f NUMERIC(10,4) DEFAULT 0`)
			safeExec(`ALTER TABLE stocks_daily_indicator ADD COLUMN IF NOT EXISTS volume_ratio NUMERIC(10,4) DEFAULT 0`)
			safeExec(`ALTER TABLE stocks_daily_indicator ADD COLUMN IF NOT EXISTS dv_ratio NUMERIC(10,4) DEFAULT 0`)
			safeExec(`ALTER TABLE stocks_daily_indicator ADD COLUMN IF NOT EXISTS dv_ttm NUMERIC(10,4) DEFAULT 0`)
			safeExec(`ALTER TABLE stocks_daily_indicator ADD COLUMN IF NOT EXISTS total_share NUMERIC(20,4) DEFAULT 0`)
			safeExec(`ALTER TABLE stocks_daily_indicator ADD COLUMN IF NOT EXISTS float_share NUMERIC(20,4) DEFAULT 0`)
			safeExec(`ALTER TABLE stocks_daily_indicator ADD COLUMN IF NOT EXISTS free_share NUMERIC(20,4) DEFAULT 0`)
			safeExec(`ALTER TABLE stocks_daily_indicator ADD COLUMN IF NOT EXISTS data_source VARCHAR(20) DEFAULT ''`)
			return nil
		},
	})


	// v050: add sector_dispersion and score_change to market_style_daily
	Register(Migration{
		Version:     50,
		Description: "PG: add sector_dispersion and score_change to market_style_daily",
		Up: func() error {
			safeExec(`ALTER TABLE market_style_daily ADD COLUMN IF NOT EXISTS sector_dispersion NUMERIC(7,4) DEFAULT 0`)
			safeExec(`ALTER TABLE market_style_daily ADD COLUMN IF NOT EXISTS score_change NUMERIC(7,2) DEFAULT 0`)
			return nil
		},
	})

	// v051: add micro-structure indicators to market_style_daily
	Register(Migration{
		Version:     51,
		Description: "PG: add break_rate, concentration, rotation_speed to market_style_daily",
		Up: func() error {
			safeExec(`ALTER TABLE market_style_daily ADD COLUMN IF NOT EXISTS break_rate NUMERIC(5,4) DEFAULT 0`)
			safeExec(`ALTER TABLE market_style_daily ADD COLUMN IF NOT EXISTS concentration NUMERIC(5,4) DEFAULT 0`)
			safeExec(`ALTER TABLE market_style_daily ADD COLUMN IF NOT EXISTS rotation_speed NUMERIC(5,4) DEFAULT 0`)
			return nil
		},
	})

	// v052: add sw_l1/sw_l2/sw_l2_dc to stocks_basic + lead_industry to market_style_daily + BK04xx reclassify
	Register(Migration{
		Version:     52,
		Description: "PG: stocks_basic sw industry fields + market_style_daily lead_industry + BK04xx reclassify",
		Up: func() error {
			safeExec(`ALTER TABLE stocks_basic ADD COLUMN IF NOT EXISTS sw_l1 VARCHAR(50) DEFAULT ''`)
			safeExec(`ALTER TABLE stocks_basic ADD COLUMN IF NOT EXISTS sw_l2 VARCHAR(50) DEFAULT ''`)
			safeExec(`ALTER TABLE stocks_basic ADD COLUMN IF NOT EXISTS sw_l2_dc VARCHAR(50) DEFAULT ''`)
			safeExec(`ALTER TABLE market_style_daily ADD COLUMN IF NOT EXISTS lead_industry VARCHAR(100) DEFAULT ''`)

			// Reclassify BK04xx real industries → industry_l2 (exclude concept-like)
			safeExec(`UPDATE concept_boards SET concept_type = 'industry_l2'
				WHERE concept_code LIKE 'BK04%'
				  AND concept_name NOT IN ('军工','节能环保','新能源','AH股','AB股','煤化工概念','酿酒概念')`)
			safeExec(`UPDATE stock_concepts SET concept_type = 'industry_l2'
				WHERE concept_code IN (SELECT concept_code FROM concept_boards WHERE concept_type = 'industry_l2')`)
			return nil
		},
	})

	// v053: Live trading tables
	Register(Migration{
		Version:     53,
		Description: "MySQL: live trading tables (fund_allocations, live_positions, live_trades, daily_portfolio_snapshots)",
		Up: func() error {
			gormAutoMigrate(MySQL,
				&model.StrategyFundAllocation{},
				&model.LivePosition{},
				&model.LiveTrade{},
				&model.DailyPortfolioSnapshot{},
			)
			return nil
		},
	})


	// v054: Multi-account support — alter trading_accounts
	Register(Migration{
		Version:     54,
		Description: "MySQL: gorm auto-migrate trading_accounts for new columns + drop unique user_id",
		Up: func() error {
			gormAutoMigrate(MySQL, &model.TradingAccount{})
			// GORM's auto-migrate keeps the old unique index; we need to replace it with a normal index
			MySQL.Exec(`ALTER TABLE trading_accounts DROP INDEX IF EXISTS idx_trading_accounts_user_id`)
			MySQL.Exec(`CREATE INDEX IF NOT EXISTS idx_trading_accounts_user_id ON trading_accounts(user_id)`)
			return nil
		},
	})

	// v055: Pre-market decision records
	Register(Migration{
		Version:     55,
		Description: "MySQL: create pre_market_decisions table",
		Up: func() error {
			gormAutoMigrate(MySQL, &model.PreMarketDecision{})
			return nil
		},
	})

	// v056: Notification configs and logs
	Register(Migration{
		Version:     56,
		Description: "MySQL: create notification_configs and notifications tables",
		Up: func() error {
			gormAutoMigrate(MySQL, &model.NotificationConfig{}, &model.Notification{})
			return nil
		},
	})

	// v057: Add account_id to holdings + backfill historical data
	Register(Migration{
		Version:     57,
		Description: "MySQL: add account_id to holdings, create default real account, backfill history",
		Up: func() error {
			MySQL.Exec(`ALTER TABLE holdings ADD COLUMN IF NOT EXISTS account_id INT UNSIGNED DEFAULT 0`)
			MySQL.Exec(`CREATE INDEX IF NOT EXISTS idx_holdings_account_id ON holdings(account_id)`)

			// Create default "真实账户" for users who have holdings but no trading account
			rows, _ := MySQL.Raw(`SELECT DISTINCT user_id FROM holdings WHERE account_id = 0`).Rows()
			if rows != nil {
				var userIDs []uint
				for rows.Next() {
					var uid uint
					rows.Scan(&uid)
					userIDs = append(userIDs, uid)
				}
				rows.Close()

				for _, uid := range userIDs {
					// Check if user already has any trading account
					var count int64
					MySQL.Raw(`SELECT COUNT(*) FROM trading_accounts WHERE user_id = ? AND status = 'active'`, uid).Scan(&count)
					if count == 0 {
						// Create default real account
						var totalCost float64
						MySQL.Raw(`SELECT COALESCE(SUM(total_cost), 0) FROM holdings WHERE user_id = ?`, uid).Scan(&totalCost)
						initialCapital := totalCost + 100000.0 // cost + some buffer
						MySQL.Exec(`INSERT INTO trading_accounts (user_id, name, broker, account_type, account_number, initial_capital, available_cash, total_deposit, status, created_at, updated_at)
							VALUES (?, '历史真实账户', '默认券商', 'real', '', ?, ?, ?, 'active', NOW(), NOW())`,
							uid, initialCapital, initialCapital - totalCost, initialCapital)
					}

					// Get the user's default account
					var accountID uint
					MySQL.Raw(`SELECT id FROM trading_accounts WHERE user_id = ? AND status = 'active' ORDER BY id ASC LIMIT 1`, uid).Scan(&accountID)

					// Backfill historical holdings
					if accountID > 0 {
						MySQL.Exec(`UPDATE holdings SET account_id = ? WHERE user_id = ? AND account_id = 0`, accountID, uid)
					}
				}
			}
			return nil
		},
	})

	// v058: Add last_run_log to strategy_runs
	Register(Migration{
		Version:     58,
		Description: "MySQL: add last_run_log TEXT column to strategy_runs",
		Up: func() error {
			gormAutoMigrate(MySQL, &model.StrategyRun{})
			return nil
		},
	})

	// v059: PreMarketTask for async decision pipeline
	Register(Migration{
		Version:     59,
		Description: "MySQL: pre_market_tasks table for async decision pipeline",
		Up: func() error {
			gormAutoMigrate(MySQL, &model.PreMarketTask{})
			return nil
		},
	})

	// v061: Add mx_moni fields to trading_accounts
	Register(Migration{
		Version:     61,
		Description: "MySQL: add mx_moni broker fields to trading_accounts",
		Up: func() error {
			MySQL.Exec("ALTER TABLE trading_accounts ADD COLUMN mx_api_key VARCHAR(200) DEFAULT '' AFTER status")
			MySQL.Exec("ALTER TABLE trading_accounts ADD COLUMN mx_account_id VARCHAR(50) DEFAULT '' AFTER mx_api_key")
			MySQL.Exec("ALTER TABLE trading_accounts ADD COLUMN broker_mode VARCHAR(20) DEFAULT 'manual' AFTER mx_account_id")
			return nil
		},
	})

	// v063: Add dynamic position sizing fields to strategies
	Register(Migration{
		Version:     63,
		Description: "MySQL: add position sizing config fields to strategies",
		Up: func() error {
			MySQL.Exec("ALTER TABLE strategies ADD COLUMN enable_dynamic_sizing TINYINT(1) DEFAULT 1 AFTER max_daily_loss")
			MySQL.Exec("ALTER TABLE strategies ADD COLUMN max_total_position DOUBLE DEFAULT 0 AFTER enable_dynamic_sizing")
			MySQL.Exec("ALTER TABLE strategies ADD COLUMN daily_buy_limit DOUBLE DEFAULT 0 AFTER max_total_position")
			MySQL.Exec("ALTER TABLE strategies ADD COLUMN max_single_industry DOUBLE DEFAULT 30 AFTER daily_buy_limit")
			MySQL.Exec("ALTER TABLE strategies ADD COLUMN min_industry_count INT DEFAULT 3 AFTER max_single_industry")
			MySQL.Exec("ALTER TABLE strategies ADD COLUMN enable_sector_rotation TINYINT(1) DEFAULT 1 AFTER min_industry_count")
			MySQL.Exec("ALTER TABLE strategies ADD COLUMN enable_theme_overweight TINYINT(1) DEFAULT 1 AFTER enable_sector_rotation")
			return nil
		},
	})

	// v064: Add scoring_config to strategies
	Register(Migration{
		Version:     64,
		Description: "MySQL: add scoring_config JSON to strategies",
		Up: func() error {
			MySQL.Exec("ALTER TABLE strategies ADD COLUMN scoring_config JSON AFTER enable_theme_overweight")
			return nil
		},
	})

	// v068: Remove dead ai_agent_mode column from strategies
	Register(Migration{
		Version:     71,
		Description: "MySQL: remove ai_agent_mode from strategies",
		Up: func() error {
			MySQL.Exec("ALTER TABLE strategies DROP COLUMN IF EXISTS ai_agent_mode")
			return nil
		},
	})

	// v069: Add execution_mode to strategy_runs (force via ALTER TABLE)
	Register(Migration{
		Version:     69,
		Description: "MySQL: add execution_mode to strategy_runs via ALTER TABLE",
		Up: func() error {
			MySQL.Exec("ALTER TABLE strategy_runs ADD COLUMN execution_mode VARCHAR(20) DEFAULT 'manual' AFTER auto_pre_market_cron")
			return nil
		},
	})

	// v068: Add execution_mode to strategy_runs
	Register(Migration{
		Version:     71,
		Description: "MySQL: add execution_mode to strategy_runs",
		Up: func() error {
			gormAutoMigrate(MySQL, &model.StrategyRun{})
			return nil
		},
	})

	// v067: Add run_id to pre_market_tasks
	Register(Migration{
		Version:     67,
		Description: "MySQL: add run_id to pre_market_tasks",
		Up: func() error {
			MySQL.Exec("ALTER TABLE pre_market_tasks ADD COLUMN run_id INT UNSIGNED DEFAULT 0 AFTER user_id")
			return nil
		},
	})

	// v066: Add skip_ai to pre_market_tasks
	Register(Migration{
		Version:     66,
		Description: "MySQL: add skip_ai to pre_market_tasks",
		Up: func() error {
			MySQL.Exec("ALTER TABLE pre_market_tasks ADD COLUMN skip_ai TINYINT(1) DEFAULT 0 AFTER result_json")
			return nil
		},
	})

	// v062: Add broker-aligned financial fields to trading_accounts

	// v068: Remove dead ai_agent_mode column from strategies
	Register(Migration{
		Version:     71,
		Description: "MySQL: remove ai_agent_mode from strategies",
		Up: func() error {
			MySQL.Exec("ALTER TABLE strategies DROP COLUMN IF EXISTS ai_agent_mode")
			return nil
		},
	})

	// v069: Add execution_mode to strategy_runs (force via ALTER TABLE)
	Register(Migration{
		Version:     69,
		Description: "MySQL: add execution_mode to strategy_runs via ALTER TABLE",
		Up: func() error {
			MySQL.Exec("ALTER TABLE strategy_runs ADD COLUMN execution_mode VARCHAR(20) DEFAULT 'manual' AFTER auto_pre_market_cron")
			return nil
		},
	})

	// v068: Add execution_mode to strategy_runs
	Register(Migration{
		Version:     71,
		Description: "MySQL: add execution_mode to strategy_runs",
		Up: func() error {
			gormAutoMigrate(MySQL, &model.StrategyRun{})
			return nil
		},
	})

	// v067: Add run_id to pre_market_tasks
	Register(Migration{
		Version:     67,
		Description: "MySQL: add run_id to pre_market_tasks",
		Up: func() error {
			MySQL.Exec("ALTER TABLE pre_market_tasks ADD COLUMN run_id INT UNSIGNED DEFAULT 0 AFTER user_id")
			return nil
		},
	})

	// v066: Add skip_ai to pre_market_tasks
	Register(Migration{
		Version:     66,
		Description: "MySQL: add skip_ai to pre_market_tasks",
		Up: func() error {
			MySQL.Exec("ALTER TABLE pre_market_tasks ADD COLUMN skip_ai TINYINT(1) DEFAULT 0 AFTER result_json")
			return nil
		},
	})

	// v062: Add broker-aligned financial fields to trading_accounts
	Register(Migration{
		Version:     62,
		Description: "MySQL: add total_assets, total_market_value, total_profit, nav to trading_accounts",
		Up: func() error {
			MySQL.Exec("ALTER TABLE trading_accounts ADD COLUMN total_assets NUMERIC(16,2) DEFAULT 0 AFTER frozen_cash")
			MySQL.Exec("ALTER TABLE trading_accounts ADD COLUMN total_market_value NUMERIC(16,2) DEFAULT 0 AFTER total_assets")
			MySQL.Exec("ALTER TABLE trading_accounts ADD COLUMN total_profit NUMERIC(16,2) DEFAULT 0 AFTER total_market_value")
			MySQL.Exec("ALTER TABLE trading_accounts ADD COLUMN nav NUMERIC(10,4) DEFAULT 1 AFTER total_profit")
			return nil
		},
	})

	// v065: Add daily_run_tasks for async execution tracking
	Register(Migration{
		Version:     65,
		Description: "MySQL: daily_run_tasks table for async daily-run tracking",
		Up: func() error {
			gormAutoMigrate(MySQL, &model.DailyRunTask{})
			return nil
		},
	})

	// v060: Add run_id to backtest_signals for multi-run isolation
	Register(Migration{
		Version:     60,
		Description: "MySQL: add run_id to backtest_signals",
		Up: func() error {
			// MySQL 5.7 compatible: check if column exists before adding
			var count int64
			MySQL.Raw("SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = 'stock_predict' AND TABLE_NAME = 'backtest_signals' AND COLUMN_NAME = 'run_id'").Scan(&count)
			if count == 0 {
				MySQL.Exec("ALTER TABLE backtest_signals ADD COLUMN run_id INT UNSIGNED NOT NULL DEFAULT 0 AFTER task_id")
			}
			MySQL.Exec("CREATE INDEX idx_sig_run ON backtest_signals(run_id)")
			return nil
		},
	})
	// v70: rename pre_market → trade_exec + ai_review + account_id + charset
	Register(Migration{
		Version:     71,
		Description: "MySQL v71: trade_exec rename + ai_review + account_id + charset",
		Up: func() error {
			// Rename column (ignore error if already renamed)
			MySQL.Exec("ALTER TABLE strategy_runs CHANGE COLUMN auto_pre_market_cron auto_trade_exec_cron VARCHAR(50)")
			// Add ai_review_enabled if not exists
			var count int64
			MySQL.Raw("SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = 'stock_predict' AND TABLE_NAME = 'strategy_runs' AND COLUMN_NAME = 'ai_review_enabled'").Scan(&count)
			if count == 0 {
				MySQL.Exec("ALTER TABLE strategy_runs ADD COLUMN ai_review_enabled TINYINT(1) DEFAULT 0 AFTER execution_mode")
			}
			// Add account_id if not exists
			MySQL.Raw("SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = 'stock_predict' AND TABLE_NAME = 'strategy_runs' AND COLUMN_NAME = 'account_id'").Scan(&count)
			if count == 0 {
				MySQL.Exec("ALTER TABLE strategy_runs ADD COLUMN account_id INT UNSIGNED DEFAULT 0 AFTER name")
			}
			// Fix charset for key tables
			MySQL.Exec("ALTER TABLE strategy_runs CONVERT TO CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci")
			MySQL.Exec("ALTER TABLE trading_accounts CONVERT TO CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci")
			MySQL.Exec("ALTER TABLE strategies CONVERT TO CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci")
			MySQL.Exec("ALTER TABLE backtest_signals CONVERT TO CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci")
			// Backfill account_id from fund allocations
			MySQL.Exec("UPDATE strategy_runs sr JOIN strategy_fund_allocations sfa ON sfa.strategy_run_id = sr.id AND sfa.status = 'active' SET sr.account_id = sfa.trading_account_id WHERE sr.account_id = 0")
			return nil
		},
	})

	// v72: backfill account_id (fix column name from v71)
	Register(Migration{
		Version:     72,
		Description: "MySQL: backfill account_id from fund allocations",
		Up: func() error {
			MySQL.Exec("UPDATE strategy_runs sr JOIN strategy_fund_allocations sfa ON sfa.strategy_run_id = sr.id AND sfa.status = 'active' SET sr.account_id = sfa.account_id WHERE sr.account_id = 0")
			return nil
		},
	})

	// v73: extend status column for live trading statuses (pending_order, pending_manual, order_failed)
	Register(Migration{
		Version:     73,
		Description: "MySQL: extend backtest_signals.status to VARCHAR(30)",
		Up: func() error {
			return MySQL.Exec("ALTER TABLE backtest_signals MODIFY COLUMN status VARCHAR(30) NOT NULL DEFAULT 'pending'").Error
		},
	})

	// v74: add broker_order_id column for order status tracking
	Register(Migration{
		Version:     74,
		Description: "MySQL: add broker_order_id to backtest_signals",
		Up: func() error {
			return MySQL.Exec("ALTER TABLE backtest_signals ADD COLUMN broker_order_id VARCHAR(30) DEFAULT '' AFTER skip_reason").Error
		},
	})

	// v75: create run_execution_logs for per-day log storage
	Register(Migration{
		Version:     75,
		Description: "MySQL: create run_execution_logs table",
		Up: func() error {
			return MySQL.Exec(`
				CREATE TABLE IF NOT EXISTS run_execution_logs (
					id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
					run_id INT UNSIGNED NOT NULL,
					trade_date VARCHAR(10) NOT NULL,
					log_type VARCHAR(20) NOT NULL DEFAULT 'strategy',
					level VARCHAR(10) DEFAULT 'info',
					stock_code VARCHAR(10) DEFAULT '',
					stock_name VARCHAR(50) DEFAULT '',
					message VARCHAR(2000),
					detail TEXT,
					created_at DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3),
					INDEX idx_run_date (run_id, trade_date),
					INDEX idx_run_type (run_id, log_type, trade_date)
				) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4
			`).Error
		},
	})

	// v76: drop deprecated auto_pre_market_cron column (safe, won't block on missing)
	Register(Migration{
		Version:     76,
		Description: "MySQL: drop deprecated auto_pre_market_cron from strategy_runs",
		Up: func() error {
			safeExecMysql("ALTER TABLE strategy_runs DROP COLUMN IF EXISTS auto_pre_market_cron")
			return nil
		},
	})

	// v77: backtest_signals + holdings + strategies 字段补全
	Register(Migration{
		Version:     77,
		Description: "MySQL: add missing columns to backtest_signals, holdings, strategies",
		Up: func() error {
			_ = MySQL.Exec("ALTER TABLE backtest_signals ADD COLUMN suggested_premium DECIMAL(5,2) DEFAULT 0").Error
			_ = MySQL.Exec("ALTER TABLE backtest_signals ADD COLUMN order_price DECIMAL(12,4) DEFAULT 0").Error
			_ = MySQL.Exec("ALTER TABLE backtest_signals ADD COLUMN order_price_limit DECIMAL(12,4) DEFAULT 0").Error
			_ = MySQL.Exec("ALTER TABLE backtest_signals ADD COLUMN suggested_qty BIGINT DEFAULT 0").Error
			_ = MySQL.Exec("ALTER TABLE backtest_signals ADD COLUMN original_qty BIGINT DEFAULT 0").Error
			_ = MySQL.Exec("ALTER TABLE backtest_signals ADD COLUMN open_price DECIMAL(12,4) DEFAULT 0").Error
			_ = MySQL.Exec("ALTER TABLE backtest_signals ADD COLUMN open_deviation DECIMAL(6,2) DEFAULT 0").Error
			_ = MySQL.Exec("ALTER TABLE backtest_signals ADD COLUMN decision_rule VARCHAR(50) DEFAULT NULL").Error
			_ = MySQL.Exec("ALTER TABLE holdings ADD COLUMN account_id BIGINT UNSIGNED DEFAULT 0").Error
			_ = MySQL.Exec("ALTER TABLE holdings ADD COLUMN buy_date VARCHAR(10) DEFAULT NULL").Error
			_ = MySQL.Exec("ALTER TABLE holdings ADD COLUMN total_cost DECIMAL(16,2) DEFAULT 0").Error
			_ = MySQL.Exec("CREATE INDEX IF NOT EXISTS idx_holdings_account_id ON holdings(account_id)").Error
		return nil
		},
	})

	// v78: 6 个数据采集表业务唯一约束
	Register(Migration{
		Version:     78,
		Description: "PG: add business unique constraints to 6 collector tables",
		Up: func() error {
			safeExec(`ALTER TABLE ai_agent_decisions ADD CONSTRAINT IF NOT EXISTS ai_agent_decisions_business_key UNIQUE (strategy_id, trade_date)`)
			safeExec(`ALTER TABLE ai_analyses ADD CONSTRAINT IF NOT EXISTS ai_analyses_business_key UNIQUE (code, pick_date)`)
			safeExec(`ALTER TABLE ai_stock_scores ADD CONSTRAINT IF NOT EXISTS ai_stock_scores_business_key UNIQUE (code)`)
			safeExec(`ALTER TABLE block_trade ADD CONSTRAINT IF NOT EXISTS block_trade_business_key UNIQUE (code, trade_date, deal_price, deal_volume, buyer_name, seller_name)`)
			safeExec(`ALTER TABLE macro_news ADD CONSTRAINT IF NOT EXISTS macro_news_business_key UNIQUE (title, news_time, category)`)
			safeExec(`ALTER TABLE sentiment_weights ADD CONSTRAINT IF NOT EXISTS sentiment_weights_business_key UNIQUE (name)`)
			return nil
		},
	})

	// v79: market_style_daily 字段补全（风格曲线查询所需）
	Register(Migration{
		Version:     79,
		Description: "PG: add market_regime, thematic_leadership, lead_concept, growth_defense_flow to market_style_daily",
		Up: func() error {
			safeExec(`ALTER TABLE market_style_daily ADD COLUMN IF NOT EXISTS market_regime VARCHAR(20)`)
			safeExec(`ALTER TABLE market_style_daily ADD COLUMN IF NOT EXISTS thematic_leadership VARCHAR(20)`)
			safeExec(`ALTER TABLE market_style_daily ADD COLUMN IF NOT EXISTS lead_concept VARCHAR(100)`)
			safeExec(`ALTER TABLE market_style_daily ADD COLUMN IF NOT EXISTS growth_defense_flow DOUBLE PRECISION`)
			return nil
		},
	})



	// v80: live_positions T+1 field additions
	Register(Migration{
		Version:     80,
		Description: "MySQL: add today_buy_qty, avail_sell_qty to live_positions for T+1 enforcement",
		Up: func() error {
			_ = MySQL.Exec("ALTER TABLE live_positions ADD COLUMN today_buy_qty INT DEFAULT 0").Error
			_ = MySQL.Exec("ALTER TABLE live_positions ADD COLUMN avail_sell_qty INT DEFAULT 0").Error
			return MySQL.Exec("UPDATE live_positions SET avail_sell_qty = quantity WHERE avail_sell_qty = 0").Error
		},
	})

	// v81: holdings table additions for account-level T+1 and strategy sync
	Register(Migration{
		Version:     81,
		Description: "MySQL: add today_buy_qty, avail_sell_qty, stock_name, current_price to holdings",
		Up: func() error {
			_ = MySQL.Exec("ALTER TABLE holdings ADD COLUMN today_buy_qty INT DEFAULT 0").Error
			_ = MySQL.Exec("ALTER TABLE holdings ADD COLUMN avail_sell_qty INT DEFAULT 0").Error
			_ = MySQL.Exec("ALTER TABLE holdings ADD COLUMN stock_name VARCHAR(50) DEFAULT ''").Error
			_ = MySQL.Exec("ALTER TABLE holdings ADD COLUMN current_price DECIMAL(12,4) DEFAULT 0").Error
			// Backfill from live_positions
			return MySQL.Exec(`
				UPDATE holdings h
				INNER JOIN (
					SELECT sr.account_id, lp.stock_code COLLATE utf8mb4_unicode_ci AS stock_code,
						MAX(lp.today_buy_qty) AS tq,
						MAX(lp.avail_sell_qty) AS aq,
						MAX(lp.stock_name) AS sn,
						MAX(lp.current_price) AS cp
					FROM live_positions lp
					JOIN strategy_runs sr ON sr.id = lp.strategy_run_id
					WHERE lp.quantity > 0
					GROUP BY sr.account_id, lp.stock_code COLLATE utf8mb4_unicode_ci
				) lp ON h.stock_code COLLATE utf8mb4_unicode_ci = lp.stock_code
				SET h.today_buy_qty = COALESCE(lp.tq, 0),
				    h.avail_sell_qty = COALESCE(lp.aq, 0),
				    h.stock_name = COALESCE(lp.sn, ''),
				    h.current_price = COALESCE(lp.cp, 0)
			`).Error
		},
	})

	// v82: PG stocks_daily_k updated_at tracking
	Register(Migration{
		Version:     82,
		Description: "PG: add updated_at to stocks_daily_k for K-line freshness tracking",
		Up: func() error {
			safeExec(`ALTER TABLE stocks_daily_k ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ DEFAULT NOW()`)
			return nil
		},
	})



	// v083: Risk alert system overhaul — expanded fields, rules table, snapshots table
	Register(Migration{
		Version:     83,
		Description: "Risk alert overhaul: expand risk_alerts + add risk_rules + risk_snapshots",
		Up: func() error {
			// Expand risk_alerts (gorm AutoMigrate handles new columns)
			gormAutoMigrate(MySQL, &model.RiskAlert{})

			// Data migration: ignored → status
			MySQL.Exec("UPDATE risk_alerts SET status = 'ignored' WHERE ignored = true AND (status = 'active' OR status = '')")

			// Dedup index (safe to fail if already exists via ROW_COUNT)
			_ = MySQL.Exec("CREATE UNIQUE INDEX idx_alert_dedup ON risk_alerts(user_id, stock_code, rule_key, hit_date)")
			_ = MySQL.Exec("CREATE INDEX idx_alert_user_status ON risk_alerts(user_id, status)")
			_ = MySQL.Exec("CREATE INDEX idx_alert_rule_date ON risk_alerts(rule_key, hit_date)")

			// New tables
			gormAutoMigrate(MySQL, &model.RiskRule{})
			gormAutoMigrate(MySQL, &model.RiskSnapshot{})

			// Seed 34 risk rules
			seedRiskRules(MySQL)

			return nil
		},
	})

	// v084: macro_news 去重约束修复
	Register(Migration{
		Version:     84,
		Description: "PG: add UNIQUE constraint on macro_news(title, news_time, category) for dedup",
		Up: func() error {
			// 删除重复数据（保留最早的一条）
			PG.Exec(`
				DELETE FROM macro_news a
				USING macro_news b
				WHERE a.id > b.id
				  AND a.title = b.title
				  AND a.news_time = b.news_time
				  AND a.category = b.category
			`)
			// 删除普通索引，创建唯一索引
			_ = PG.Exec("DROP INDEX IF EXISTS idx_macro_news_time")
			_ = PG.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_macro_news_dedup ON macro_news(title, news_time, category)")
			return nil
		},
	})

	// v085: Add agent_token for lobster auto-trading local agent auth
	Register(Migration{
		Version:     85,
		Description: "MySQL: add agent_token to trading_accounts for local agent authentication",
		Up: func() error {
			MySQL.Exec("ALTER TABLE trading_accounts ADD COLUMN agent_token VARCHAR(64) DEFAULT '' AFTER broker_mode")
			return nil
		},
	})
	// v088: Drop legacy strategy_fund_allocations table (data migrated to strategy_runs in v086)
	Register(Migration{
		Version:     88,
		Description: "MySQL: drop strategy_fund_allocations table (data migrated to strategy_runs)",
		Up: func() error {
			if MySQL == nil {
				return nil
			}
			var tableExists int64
			MySQL.Raw("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'strategy_fund_allocations'").Scan(&tableExists)
			if tableExists > 0 {
				if err := MySQL.Exec("DROP TABLE IF EXISTS strategy_fund_allocations").Error; err != nil {
					log.Printf("[migrate:v088] drop strategy_fund_allocations: %v", err)
					return err
				}
				log.Printf("[migrate:v088] dropped legacy strategy_fund_allocations table")
			}
			return nil
		},
	})
	// v089: Fix corrupted strategy_runs data from old ReconcileFromBroker bug.
	// Old ReconcileFromBroker overwrote available_cash from broker without adjusting initial_capital.
	// This left runs with initial_capital > available_cash + position_value (phantom gap).
	// Fix: for runs with no cash flow history (never had real deposits/withdrawals),
	// reset initial_capital = available_cash + position_value (current real equity).
	Register(Migration{
		Version:     89,
		Description: "MySQL: fix corrupted strategy_runs initial_capital (old ReconcileFromBroker bug)",
		Up: func() error {
			if MySQL == nil {
				return nil
			}
			// Fix runs where initial_capital > available_cash + position_value
			// and no strategy_cash_flows exist (never had real deposit/withdraw history)
			result := MySQL.Exec(`
				UPDATE strategy_runs sr
				SET sr.initial_capital = sr.available_cash + COALESCE(sr.position_value, 0)
				WHERE sr.initial_capital > sr.available_cash + COALESCE(sr.position_value, 0)
				  AND sr.initial_capital > 0
				  AND sr.available_cash > 0
				  AND NOT EXISTS (
					SELECT 1 FROM strategy_cash_flows scf
					WHERE scf.strategy_run_id = sr.id
					AND scf.flow_type IN ('deposit', 'withdraw')
				  )
			`)
			if result.Error != nil {
				log.Printf("[migrate:v089] fix corrupted initial_capital: %v", result.Error)
				return result.Error
			}
			if result.RowsAffected > 0 {
				log.Printf("[migrate:v089] fixed %d corrupted strategy_runs (initial_capital reset to equity)", result.RowsAffected)
			}
			return nil
		},
	})




	// v090: API keys for external team data import
	Register(Migration{
		Version:     90,
		Description: "MySQL: api_keys table for external team data import authentication",
		Up: func() error {
			if MySQL == nil {
				return nil
			}
			safeExecMysql(`CREATE TABLE IF NOT EXISTS api_keys (
				id INT AUTO_INCREMENT PRIMARY KEY,
				key_hash VARCHAR(64) NOT NULL UNIQUE,
				key_prefix VARCHAR(12) NOT NULL DEFAULT '',
				team_name VARCHAR(100) NOT NULL DEFAULT '',
				description VARCHAR(255) NOT NULL DEFAULT '',
				permissions TEXT,
				is_active TINYINT(1) DEFAULT 1,
				last_used_at DATETIME NULL,
				created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
				updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
				INDEX idx_api_keys_key_prefix (key_prefix),
				INDEX idx_api_keys_is_active (is_active)
			) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`)
			return nil
		},
	})

}
