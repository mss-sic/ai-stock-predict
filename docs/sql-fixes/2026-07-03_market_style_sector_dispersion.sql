-- Add sector_dispersion and score_change to market_style_daily
-- For structural divergence detection in market style classification
-- Migration v050

ALTER TABLE market_style_daily ADD COLUMN IF NOT EXISTS sector_dispersion NUMERIC(7,4) DEFAULT 0;
ALTER TABLE market_style_daily ADD COLUMN IF NOT EXISTS score_change NUMERIC(7,2) DEFAULT 0;

-- Add micro-structure indicators to market_style_daily (v051)
ALTER TABLE market_style_daily ADD COLUMN IF NOT EXISTS break_rate NUMERIC(5,4) DEFAULT 0;
ALTER TABLE market_style_daily ADD COLUMN IF NOT EXISTS concentration NUMERIC(5,4) DEFAULT 0;
ALTER TABLE market_style_daily ADD COLUMN IF NOT EXISTS rotation_speed NUMERIC(5,4) DEFAULT 0;

-- Add market_style AI scene config (for AI auto-summary)
-- Uses agent_* fields to bypass user auth for system calls
INSERT INTO ai_system_configs (scene, name, system_prompt, agent_model_name, agent_base_url, agent_api_key, temperature, max_tokens)
VALUES ('market_style', '市场风格AI分析', 
'你是A股市场分析师。基于数据生成80-120字市场日评。格式：一段话，包含[风格诊断] + [关键信号] + [关注方向] + [风险提示]。简洁专业。',
'deepseek-chat', 'https://api.deepseek.com', '', 0.5, 256)
ON CONFLICT (scene) DO NOTHING;
