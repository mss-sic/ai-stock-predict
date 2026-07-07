-- ============================================
-- PostgreSQL 线上去重脚本 (2026-07-08)
-- 先检查重复量，再分表去重
-- 策略: 每组重复保留最大 id，删除其余
-- ============================================

-- ════════════════════════════════════════════
-- Step 0: 检查各表重复数量 (只读，放心执行)
-- ════════════════════════════════════════════

SELECT 'ai_agent_decisions' as tbl, count(*) as total_rows,
  (SELECT count(*) FROM (SELECT 1 FROM ai_agent_decisions GROUP BY strategy_id, trade_date HAVING count(*) > 1) t) as dup_groups
FROM ai_agent_decisions;

SELECT 'ai_analyses' as tbl, count(*) as total_rows,
  (SELECT count(*) FROM (SELECT 1 FROM ai_analyses GROUP BY code, pick_date HAVING count(*) > 1) t) as dup_groups
FROM ai_analyses;

SELECT 'ai_stock_scores' as tbl, count(*) as total_rows,
  (SELECT count(*) FROM (SELECT 1 FROM ai_stock_scores GROUP BY code HAVING count(*) > 1) t) as dup_groups
FROM ai_stock_scores;

SELECT 'block_trade' as tbl, count(*) as total_rows,
  (SELECT count(*) FROM (SELECT 1 FROM block_trade GROUP BY code, trade_date, deal_price, deal_volume, buyer_name, seller_name HAVING count(*) > 1) t) as dup_groups
FROM block_trade;

SELECT 'macro_news' as tbl, count(*) as total_rows,
  (SELECT count(*) FROM (SELECT 1 FROM macro_news GROUP BY title, news_time, category HAVING count(*) > 1) t) as dup_groups
FROM macro_news;

SELECT 'sentiment_weights' as tbl, count(*) as total_rows,
  (SELECT count(*) FROM (SELECT 1 FROM sentiment_weights GROUP BY name HAVING count(*) > 1) t) as dup_groups
FROM sentiment_weights;

-- ════════════════════════════════════════════
-- Step 1: 去重 (确认Step 0后再执行)
-- ════════════════════════════════════════════

-- 1. ai_agent_decisions: 同 strategy+同日期 保留最新 id
DELETE FROM ai_agent_decisions a
USING ai_agent_decisions b
WHERE a.strategy_id = b.strategy_id
  AND a.trade_date = b.trade_date
  AND a.id < b.id;

-- 2. ai_analyses: 同股票+同日期 保留最新 id
DELETE FROM ai_analyses a
USING ai_analyses b
WHERE a.code = b.code
  AND a.pick_date = b.pick_date
  AND a.id < b.id;

-- 3. ai_stock_scores: 同股票 保留最新 id
DELETE FROM ai_stock_scores a
USING ai_stock_scores b
WHERE a.code = b.code AND a.id < b.id;

-- 4. block_trade: 同笔大宗交易 保留最新 id
DELETE FROM block_trade a
USING block_trade b
WHERE a.code = b.code
  AND a.trade_date = b.trade_date
  AND a.deal_price = b.deal_price
  AND a.deal_volume = b.deal_volume
  AND a.buyer_name = b.buyer_name
  AND a.seller_name = b.seller_name
  AND a.id < b.id;

-- 5. macro_news: 同标题+同时间+同分类 保留最新 id
DELETE FROM macro_news a
USING macro_news b
WHERE a.title = b.title
  AND a.news_time = b.news_time
  AND a.category = b.category
  AND a.id < b.id;

-- 6. sentiment_weights: 同名方案 保留最新 id
DELETE FROM sentiment_weights a
USING sentiment_weights b
WHERE a.name = b.name AND a.id < b.id;

-- ════════════════════════════════════════════
-- Step 2: 验证重复已清零
-- ════════════════════════════════════════════

SELECT 'ai_agent_decisions' as tbl, count(*) as remaining_dups
FROM (SELECT strategy_id, trade_date, count(*) as n FROM ai_agent_decisions GROUP BY 1,2 HAVING count(*) > 1) t;

SELECT 'ai_analyses' as tbl, count(*) as remaining_dups
FROM (SELECT code, pick_date, count(*) as n FROM ai_analyses GROUP BY 1,2 HAVING count(*) > 1) t;

SELECT 'ai_stock_scores' as tbl, count(*) as remaining_dups
FROM (SELECT code, count(*) as n FROM ai_stock_scores GROUP BY 1 HAVING count(*) > 1) t;

SELECT 'block_trade' as tbl, count(*) as remaining_dups
FROM (SELECT code, trade_date, deal_price, deal_volume, buyer_name, seller_name, count(*) as n FROM block_trade GROUP BY 1,2,3,4,5,6 HAVING count(*) > 1) t;

SELECT 'macro_news' as tbl, count(*) as remaining_dups
FROM (SELECT title, news_time, category, count(*) as n FROM macro_news GROUP BY 1,2,3 HAVING count(*) > 1) t;

SELECT 'sentiment_weights' as tbl, count(*) as remaining_dups
FROM (SELECT name, count(*) as n FROM sentiment_weights GROUP BY 1 HAVING count(*) > 1) t;
