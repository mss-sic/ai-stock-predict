-- ============================================
-- 线上数据库字段 & 索引修复 (2026-07-08)
-- Go 模型 vs 线上 DB 对比结果
-- ============================================

-- --------------------------------------------------
-- [backtest_signals] 8 个缺失字段
-- 来源: model/backtest_signal.go (v1.9.0 交易执行增强)
-- --------------------------------------------------
ALTER TABLE `backtest_signals` ADD COLUMN `suggested_premium` decimal(5,2) DEFAULT 0 COMMENT '建议溢价百分比(2.5=2.5%)';
ALTER TABLE `backtest_signals` ADD COLUMN `order_price` decimal(12,4) DEFAULT 0 COMMENT '建议委托价';
ALTER TABLE `backtest_signals` ADD COLUMN `order_price_limit` decimal(12,4) DEFAULT 0 COMMENT '价格上限(买)/下限(卖)';
ALTER TABLE `backtest_signals` ADD COLUMN `suggested_qty` bigint DEFAULT 0 COMMENT '波动率调整后建议数量';
ALTER TABLE `backtest_signals` ADD COLUMN `original_qty` bigint DEFAULT 0 COMMENT '调整前原始计划数量';
ALTER TABLE `backtest_signals` ADD COLUMN `open_price` decimal(12,4) DEFAULT 0 COMMENT '当日开盘价';
ALTER TABLE `backtest_signals` ADD COLUMN `open_deviation` decimal(6,2) DEFAULT 0 COMMENT '开盘偏离百分比';
ALTER TABLE `backtest_signals` ADD COLUMN `decision_rule` varchar(50) DEFAULT NULL COMMENT '触发的决策矩阵规则';

-- --------------------------------------------------
-- [holdings] 3 个缺失字段 + 1 个缺失索引
-- 来源: model/holding.go (v1.9.0 交易账户绑定)
-- --------------------------------------------------
ALTER TABLE `holdings` ADD COLUMN `account_id` bigint unsigned DEFAULT 0 COMMENT '交易账户ID';
ALTER TABLE `holdings` ADD COLUMN `buy_date` varchar(10) DEFAULT NULL COMMENT '买入日期(YYYY-MM-DD)';
ALTER TABLE `holdings` ADD COLUMN `total_cost` decimal(16,2) DEFAULT 0 COMMENT '总成本=costPrice×quantity';
ALTER TABLE `holdings` ADD INDEX `idx_holdings_account_id` (`account_id`);

-- --------------------------------------------------
-- [strategies] 4 个缺失字段
-- 来源: model/strategy.go (v1.9.0 策略元数据增强)
-- --------------------------------------------------
ALTER TABLE `strategies` ADD COLUMN `expected_hold_days` bigint DEFAULT 5 COMMENT '策略目标持仓天数';
ALTER TABLE `strategies` ADD COLUMN `risk_profile` varchar(30) DEFAULT NULL COMMENT '风险偏好:aggressive/balanced/conservative';
ALTER TABLE `strategies` ADD COLUMN `strategy_style` varchar(30) DEFAULT NULL COMMENT '策略风格:momentum/swing/trend/value/dip/grid';
ALTER TABLE `strategies` ADD COLUMN `strategy_thesis` varchar(500) DEFAULT NULL COMMENT '用户自述策略理念';
