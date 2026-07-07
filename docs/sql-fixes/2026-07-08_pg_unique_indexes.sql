-- ============================================
-- PostgreSQL public schema 唯一约束修复 (2026-07-08)
-- 确保数据采集表有正确的唯一索引，防止重复数据
-- ============================================

-- 1. ai_agent_decisions: 每个策略每天只有一条决策
ALTER TABLE "ai_agent_decisions" ADD CONSTRAINT "ai_agent_decisions_business_key" UNIQUE ("strategy_id", "trade_date");

-- 2. ai_analyses: 每只股票每天一条分析
ALTER TABLE "ai_analyses" ADD CONSTRAINT "ai_analyses_business_key" UNIQUE ("code", "pick_date");

-- 3. ai_stock_scores: 每只股票只有最新评分
ALTER TABLE "ai_stock_scores" ADD CONSTRAINT "ai_stock_scores_business_key" UNIQUE ("code");

-- 4. block_trade: 大宗交易唯一记录（代码+日期+价格+量+买卖方）
-- INSERT 列: code, trade_date, deal_price, close_price, premium_pct, deal_volume, deal_amt, buyer_name, seller_name
ALTER TABLE "block_trade" ADD CONSTRAINT "block_trade_business_key" 
    UNIQUE ("code", "trade_date", "deal_price", "deal_volume", "buyer_name", "seller_name");

-- 5. macro_news: 宏观新闻去重（标题+时间+分类）
-- INSERT 列: title, summary, news_time, category
ALTER TABLE "macro_news" ADD CONSTRAINT "macro_news_business_key" 
    UNIQUE ("title", "news_time", "category");

-- 6. sentiment_weights: 权重方案名称唯一
ALTER TABLE "sentiment_weights" ADD CONSTRAINT "sentiment_weights_business_key" UNIQUE ("name");

-- ============================================
-- 验证: 以下表已有正确的唯一约束，无需处理
-- ============================================
-- stocks_daily_k:           PK(code, trade_date) ✅
-- stocks_daily_indicator:   PK(code, trade_date) ✅
-- stock_capital_flow:       PK(code, trade_date) ✅
-- stock_financials:         UNIQUE(code, report_date) ✅
-- stock_fund_flow:          UNIQUE INDEX(code, trade_date) ✅
-- stock_concepts:           UNIQUE(code, concept_code) ✅
-- stock_shareholders:       UNIQUE(code, report_date) ✅
-- stock_news:               UNIQUE(code, title, publish_date) ✅
-- stock_profiles:           UNIQUE INDEX(code) ✅
-- stock_reports:            UNIQUE(info_code) ✅
-- stocks_basic:             PK(code) ✅
-- stock_realtime_quote:     PK(code) ✅
-- concept_boards:           PK(concept_code) ✅
-- concept_analyses:         UNIQUE(concept_code) ✅
-- dividend_history:         UNIQUE INDEX(code, ex_dividend_date) ✅
-- dragon_tiger_list:        UNIQUE INDEX(code, trade_date) ✅
-- dragon_tiger_detail:      UNIQUE INDEX(code, trade_date, seat_name, net_amt) ✅
-- margin_trading:           UNIQUE INDEX(code, trade_date) ✅
-- restricted_share_unlock:  UNIQUE INDEX(code, free_date, stock_type) ✅
-- cninfo_announcements:     UNIQUE INDEX(code, title, ann_date) ✅
-- ths_eps_forecast:         UNIQUE INDEX(code, year) ✅
-- ths_hot_stocks:           UNIQUE INDEX(code, trade_date) ✅
-- predictions:              UNIQUE INDEX(code, model_name, predict_date) ✅
-- prediction_kdist:         UNIQUE(code) ✅
-- ai_system_configs:        UNIQUE INDEX(scene) ✅
-- algorithm_picks:          UNIQUE(pick_date) ✅
-- algorithm_pick_details:   UNIQUE(pick_date, stock_code) ✅
-- market_daily_agg:         PK(trade_date) ✅
-- market_sentiment:         PK(trade_date) ✅
-- market_style_daily:       PK(trade_date) ✅
-- northbound_flow:          PK(trade_date) ✅
-- northbound_minute:        PK(trade_date, time) ✅
-- limit_stats_daily:        UNIQUE INDEX(trade_date) ✅
-- schema_migrations:        PK(version) ✅
-- stock_quotes:             UNIQUE(code) ✅
-- stock_signals:            UNIQUE(code) ✅
