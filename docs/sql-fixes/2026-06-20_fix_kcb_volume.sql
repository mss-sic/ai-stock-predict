-- 修复科创板(688xxx)成交量和成交额单位错误
-- 根因: 腾讯ifzq API对科创板返回成交量单位为"股"，采集脚本错误×100
-- 影响: 609只科创板约39万行数据，总成交额虚增约53万亿
-- 执行后需重跑 precompute_aggs.py 和 compute_sentiment.py

-- Step 1: 科创板数据修复
UPDATE stocks_daily_k SET
    volume = volume / 100,
    amount = amount / 100
WHERE code LIKE '688%' AND volume > 0;

-- Step 2: 验证 (期望: 总和约3.3万亿)
SELECT trade_date, COUNT(DISTINCT code), SUM(amount)/1e12 as 万亿
FROM stocks_daily_k
WHERE trade_date = CURRENT_DATE - 1
  AND code NOT LIKE 'IDX%'
GROUP BY trade_date;
