-- v025: ETF/国债 board_type 回填 + stocks_basic 补充
-- 生产环境执行此脚本

-- 1. 回填 ETF board_type
UPDATE stocks_basic SET board_type = 'bond' WHERE code LIKE '511%' AND board_type IS NULL;
UPDATE stocks_basic SET board_type = 'bond' WHERE code LIKE '1596%' AND board_type IS NULL;
UPDATE stocks_basic SET board_type = 'etf' WHERE code LIKE '51%' AND board_type IS NULL AND code !~ '^IDX';
UPDATE stocks_basic SET board_type = 'etf' WHERE code LIKE '159%' AND board_type IS NULL AND code !~ '^IDX';
UPDATE stocks_basic SET board_type = 'etf' WHERE code LIKE '56%' AND board_type IS NULL AND code !~ '^IDX';
UPDATE stocks_basic SET board_type = 'etf' WHERE code LIKE '58%' AND board_type IS NULL AND code !~ '^IDX';

-- 2. 补充 stocks_basic 中缺失的债券 ETF（K线已在 stocks_daily_k 中）
INSERT INTO stocks_basic (code, name, board_type, is_st, updated_at) VALUES
('511010', '国债ETF', 'bond', false, NOW()),
('511090', '30年国债ETF', 'bond', false, NOW()),
('511520', '政金债券ETF', 'bond', false, NOW())
ON CONFLICT (code) DO UPDATE SET board_type = 'bond', updated_at = NOW();

-- 3. 验证
SELECT board_type, COUNT(*) FROM stocks_basic GROUP BY board_type ORDER BY board_type;
