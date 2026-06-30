package service

import "github.com/ai-stock-predict/server/internal/db"

// CapitalSummary is the top-level capital flow overview.
type CapitalSummary struct {
	NorthboundNet  float64 `json:"northboundNet"`
	NorthboundDate string  `json:"northboundDate"`
	FundFlowMain   float64 `json:"fundFlowMain"`
	FundFlowDate   string  `json:"fundFlowDate"`
	MarginBalance  float64 `json:"marginBalance"`
	MarginDate     string  `json:"marginDate"`
	DragonTigerCnt int     `json:"dragonTigerCnt"`
	BlockTradeCnt  int     `json:"blockTradeCnt"`
}

// GetCapitalSummary fetches aggregated capital flow overview.
func GetCapitalSummary() (*CapitalSummary, error) {
	var s CapitalSummary

	// Northbound latest
	db.PG.Raw(`SELECT COALESCE(total_net,0), trade_date::text FROM northbound_daily_view ORDER BY trade_date DESC LIMIT 1`).
		Row().Scan(&s.NorthboundNet, &s.NorthboundDate)

	// Fund flow aggregate for latest day
	db.PG.Raw(`
		SELECT COALESCE(SUM(main_net),0)/1e4, trade_date::text FROM stock_fund_flow
		WHERE trade_date = (SELECT MAX(trade_date) FROM stock_fund_flow)
		GROUP BY trade_date
	`).Row().Scan(&s.FundFlowMain, &s.FundFlowDate)

	// Margin balance
	db.PG.Raw(`SELECT COALESCE(SUM(rzye),0)/1e8, trade_date::text FROM margin_trading WHERE trade_date = (SELECT MAX(trade_date) FROM margin_trading) GROUP BY trade_date`).
		Row().Scan(&s.MarginBalance, &s.MarginDate)

	// Dragon tiger count
	db.PG.Raw(`SELECT COUNT(*) FROM dragon_tiger_list WHERE trade_date = (SELECT MAX(trade_date) FROM dragon_tiger_list)`).
		Row().Scan(&s.DragonTigerCnt)

	// Block trade count
	db.PG.Raw(`SELECT COUNT(*) FROM block_trade WHERE trade_date = (SELECT MAX(trade_date) FROM block_trade)`).
		Row().Scan(&s.BlockTradeCnt)

	return &s, nil
}

// FundFlowRank is a single stock's fund flow entry.
type FundFlowRank struct {
	Code      string  `json:"code"`
	Name      string  `json:"name"`
	MainNet   float64 `json:"mainNet"`
	SuperNet  float64 `json:"superNet"`
	TotalNet  float64 `json:"totalNet"`
	TradeDate string  `json:"tradeDate"`
}

// GetFundFlowTop returns top N stocks by main net inflow on latest day.
func GetFundFlowTop(limit int, direction string) ([]FundFlowRank, error) {
	order := "DESC"
	if direction == "out" {
		order = "ASC"
	}
	var list []FundFlowRank
	db.PG.Raw(`
		SELECT ff.code, sb.name, COALESCE(ff.main_net,0)/1e4 as main_net,
			COALESCE(ff.super_net,0)/1e4 as super_net,
			COALESCE(ff.main_net+COALESCE(ff.super_net,0),0)/1e4 as total_net,
			ff.trade_date::text
		FROM stock_fund_flow ff
		JOIN stocks_basic sb ON sb.code = ff.code
		WHERE ff.trade_date = (SELECT MAX(trade_date) FROM stock_fund_flow)
		ORDER BY ff.main_net `+order+` LIMIT ?
	`, limit).Scan(&list)
	return list, nil
}

// NorthboundTrend is daily northbound flow.
type NorthboundTrend struct {
	TradeDate string  `json:"tradeDate"`
	TotalNet  float64 `json:"totalNet"`
}

// GetNorthboundTrend returns recent northbound flow trend.
func GetNorthboundTrend(days int) ([]NorthboundTrend, error) {
	var list []NorthboundTrend
	db.PG.Raw(`SELECT trade_date::text, COALESCE(total_net,0) FROM northbound_daily_view ORDER BY trade_date DESC LIMIT ?`, days).
		Scan(&list)
	return list, nil
}

// FundFlowDaily is daily aggregate fund flow.
type FundFlowDaily struct {
	TradeDate string  `json:"tradeDate"`
	MainNet   float64 `json:"mainNet"`
	SuperNet  float64 `json:"superNet"`
	SmallNet  float64 `json:"smallNet"`
}

// GetFundFlowDaily returns daily aggregate fund flow.
func GetFundFlowDaily(days int) ([]FundFlowDaily, error) {
	var list []FundFlowDaily
	db.PG.Raw(`
		SELECT trade_date::text,
			COALESCE(SUM(main_net),0)/1e4 as main_net,
			COALESCE(SUM(super_net),0)/1e4 as super_net,
			COALESCE(SUM(small_net),0)/1e4 as small_net
		FROM stock_fund_flow
		GROUP BY trade_date ORDER BY trade_date DESC LIMIT ?
	`, days).Scan(&list)
	return list, nil
}
