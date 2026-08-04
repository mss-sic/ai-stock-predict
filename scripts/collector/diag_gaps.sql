SELECT 'K线数据范围' as item, MIN(trade_date)::text as val1, MAX(trade_date)::text as val2, COUNT(DISTINCT code)::text as val3, COUNT(*)::text as val4 FROM stocks_daily_k;

SELECT 'K线记录数分布' as item, PERCENTILE_CONT(0.1) WITHIN GROUP (ORDER BY cnt)::int::text as val1, PERCENTILE_CONT(0.5) WITHIN GROUP (ORDER BY cnt)::int::text as val2, PERCENTILE_CONT(0.9) WITHIN GROUP (ORDER BY cnt)::int::text as val3, COUNT(*) FILTER (WHERE cnt < 100)::text as val4 FROM (SELECT code, COUNT(*) as cnt FROM stocks_daily_k GROUP BY code) t;

SELECT '最新日期分布' as item, last_date::text as val1, COUNT(*)::text as val2 FROM (SELECT code, MAX(trade_date) as last_date FROM stocks_daily_k GROUP BY code) t GROUP BY last_date ORDER BY last_date DESC LIMIT 10;

SELECT '现金流覆盖率' as item, COUNT(*) FILTER (WHERE operating_cf IS NOT NULL AND operating_cf != 0)::text as val1, COUNT(*)::text as val2, ROUND(COUNT(*) FILTER (WHERE operating_cf IS NOT NULL AND operating_cf != 0)::numeric / COUNT(*)::numeric * 100, 1)::text as val3 FROM stock_financials;

SELECT '数据质量分布' as item, data_quality as val1, COUNT(*)::text as val2 FROM stocks_daily_k GROUP BY data_quality ORDER BY COUNT(*) DESC;
