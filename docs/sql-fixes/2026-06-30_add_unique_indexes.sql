-- Fix: Add unique indexes for ON CONFLICT support on collector tables
CREATE UNIQUE INDEX IF NOT EXISTS idx_dragon_tiger_list_unique ON dragon_tiger_list (code, trade_date);
CREATE UNIQUE INDEX IF NOT EXISTS idx_dragon_tiger_detail_unique ON dragon_tiger_detail (code, trade_date, seat_name, net_amt);
CREATE UNIQUE INDEX IF NOT EXISTS idx_margin_trading_unique ON margin_trading (code, trade_date);
CREATE UNIQUE INDEX IF NOT EXISTS idx_block_trade_unique ON block_trade (code, trade_date, deal_price, deal_volume);
CREATE UNIQUE INDEX IF NOT EXISTS idx_cninfo_announcements_unique ON cninfo_announcements (code, title, ann_date);
CREATE UNIQUE INDEX IF NOT EXISTS idx_macro_news_unique ON macro_news (title, news_time);
CREATE UNIQUE INDEX IF NOT EXISTS idx_dividend_history_unique ON dividend_history (code, ex_dividend_date);
CREATE UNIQUE INDEX IF NOT EXISTS idx_ths_hot_stocks_unique ON ths_hot_stocks (code, trade_date);
CREATE UNIQUE INDEX IF NOT EXISTS idx_ths_eps_forecast_unique ON ths_eps_forecast (code, year);
CREATE UNIQUE INDEX IF NOT EXISTS idx_restricted_unlock_unique ON restricted_share_unlock (code, free_date, stock_type);
