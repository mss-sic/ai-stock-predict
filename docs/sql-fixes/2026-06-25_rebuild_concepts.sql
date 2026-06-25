-- ============================================
-- 概念板块数据重建
-- 清除旧的 concept 类型数据，为 Python 重建脚本准备
-- 运行后执行: python3 scripts/collector/rebuild_concepts.py
-- ============================================

-- 清空 concept 类型映射
DELETE FROM stock_concepts WHERE concept_type = 'concept';

-- 清空 concept 类型板块
DELETE FROM concept_boards WHERE concept_type = 'concept';

-- 验证 (应有 industry 数据保留)
-- SELECT concept_type, COUNT(*) FROM stock_concepts GROUP BY concept_type;
-- SELECT concept_type, COUNT(*) FROM concept_boards GROUP BY concept_type;
