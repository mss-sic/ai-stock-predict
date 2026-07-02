package service

import (
	"fmt"

	"github.com/ai-stock-predict/server/internal/db"
)

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

func GetCapitalSummary() (*CapitalSummary, error) {
	var s CapitalSummary

	db.PG.Raw(`SELECT COALESCE(total_net,0), trade_date::text FROM northbound_daily_view ORDER BY trade_date DESC LIMIT 1`).
		Row().Scan(&s.NorthboundNet, &s.NorthboundDate)

	db.PG.Raw(`
		SELECT COALESCE(SUM((buy_vol - sell_vol) * close),0)/1e8,
			MAX(trade_date)::text
		FROM stocks_daily_k
		WHERE buy_vol > 0 AND trade_date = (SELECT MAX(trade_date) FROM stocks_daily_k WHERE buy_vol > 0)
		GROUP BY trade_date
	`).Row().Scan(&s.FundFlowMain, &s.FundFlowDate)

	db.PG.Raw(`SELECT COALESCE(SUM(rzye),0)/1e8, trade_date::text FROM margin_trading WHERE trade_date = (SELECT MAX(trade_date) FROM margin_trading) GROUP BY trade_date`).
		Row().Scan(&s.MarginBalance, &s.MarginDate)

	db.PG.Raw(`SELECT COUNT(*) FROM dragon_tiger_list WHERE trade_date = (SELECT MAX(trade_date) FROM dragon_tiger_list)`).
		Row().Scan(&s.DragonTigerCnt)

	db.PG.Raw(`SELECT COUNT(*) FROM block_trade WHERE trade_date = (SELECT MAX(trade_date) FROM block_trade)`).
		Row().Scan(&s.BlockTradeCnt)

	return &s, nil
}

// ── Net Flow Ranking ──

type NetFlowRank struct {
	Code      string  `json:"code"`
	Name      string  `json:"name"`
	NetFlow   float64 `json:"netFlow"`
	TradeDate string  `json:"tradeDate"`
}

func GetNetFlowTop(limit int, direction string) ([]NetFlowRank, error) {
	orderDir := "DESC"
	if direction == "out" {
		orderDir = "ASC"
	}
	var list []NetFlowRank
	db.PG.Raw(`
		SELECT k.code, sb.name,
			COALESCE((k.buy_vol - k.sell_vol) * k.close, 0)/1e8 as net_flow,
			k.trade_date::text
		FROM stocks_daily_k k
		JOIN stocks_basic sb ON sb.code = k.code
		WHERE k.trade_date = (SELECT MAX(trade_date) FROM stocks_daily_k WHERE buy_vol > 0)
		ORDER BY net_flow `+orderDir+` LIMIT ?
	`, limit).Scan(&list)
	return list, nil
}

type FundFlowRank struct {
	Code      string  `json:"code"`
	Name      string  `json:"name"`
	NetFlow   float64 `json:"netFlow"`
	TradeDate string  `json:"tradeDate"`
}

func GetFundFlowTop(limit int, direction string) ([]FundFlowRank, error) {
	netList, err := GetNetFlowTop(limit, direction)
	if err != nil {
		return nil, err
	}
	out := make([]FundFlowRank, len(netList))
	for i, v := range netList {
		out[i] = FundFlowRank{Code: v.Code, Name: v.Name, NetFlow: v.NetFlow, TradeDate: v.TradeDate}
	}
	return out, nil
}

// ── Daily Trend ──

type FundFlowDaily struct {
	TradeDate string  `json:"tradeDate"`
	NetFlow   float64 `json:"netFlow"`
	BuyFlow   float64 `json:"buyFlow"`
	SellFlow  float64 `json:"sellFlow"`
}

func GetFundFlowDaily(days int) ([]FundFlowDaily, error) {
	var list []FundFlowDaily
	db.PG.Raw(`
		SELECT trade_date::text,
			COALESCE(SUM((buy_vol - sell_vol) * close), 0)/1e8 as net_flow,
			COALESCE(SUM(buy_vol * close), 0)/1e8     as buy_flow,
			COALESCE(SUM(sell_vol * close), 0)/1e8    as sell_flow
		FROM stocks_daily_k
		WHERE buy_vol > 0
		GROUP BY trade_date ORDER BY trade_date DESC LIMIT ?
	`, days).Scan(&list)
	return list, nil
}

// ── Margin ──

type MarginTrend struct {
	TradeDate string  `json:"tradeDate"`
	Rzye      float64 `json:"rzye"`
	Rqye      float64 `json:"rqye"`
}

type MarginTop struct {
	Code      string  `json:"code"`
	Name      string  `json:"name"`
	Balance   float64 `json:"balance"`
	TradeDate string  `json:"tradeDate"`
}

func GetMarginTrend(days int) ([]MarginTrend, error) {
	var list []MarginTrend
	db.PG.Raw(`
		SELECT trade_date::text,
			COALESCE(SUM(rzye),0)/1e8 as rzye,
			COALESCE(SUM(rqye),0)/1e8 as rqye
		FROM margin_trading
		GROUP BY trade_date ORDER BY trade_date DESC LIMIT ?
	`, days).Scan(&list)
	return list, nil
}

func GetMarginTop(limit int, marginType string) ([]MarginTop, error) {
	col := "rzye"
	if marginType == "rq" {
		col = "rqye"
	}
	var list []MarginTop
	query := fmt.Sprintf(`
		SELECT m.code, sb.name,
			COALESCE(m.%s,0)/1e8 as balance,
			m.trade_date::text
		FROM margin_trading m
		JOIN stocks_basic sb ON sb.code = m.code
		WHERE m.trade_date = (SELECT MAX(trade_date) FROM margin_trading)
		ORDER BY m.%s DESC LIMIT ?
	`, col, col)
	db.PG.Raw(query, limit).Scan(&list)
	return list, nil
}

// ── Northbound ──

type NorthboundTrend struct {
	TradeDate string  `json:"tradeDate"`
	TotalNet  float64 `json:"totalNet"`
}

func GetNorthboundTrend(days int) ([]NorthboundTrend, error) {
	var list []NorthboundTrend
	db.PG.Raw(`SELECT trade_date::text, COALESCE(total_net,0) as total_net FROM northbound_daily_view ORDER BY trade_date DESC LIMIT ?`, days).
		Scan(&list)
	return list, nil
}

// ── 综合个股资金排名 ──

type StockCapitalRank struct {
	Code      string  `json:"code"`
	Name      string  `json:"name"`
	NetFlow   float64 `json:"netFlow"`
	Rzye      float64 `json:"rzye"`
	Rqye      float64 `json:"rqye"`
	TradeDate string  `json:"tradeDate"`
}

func GetStockCapitalRank(limit int, sortBy, order string) ([]StockCapitalRank, error) {
	orderCol := "net_flow"
	orderDir := "DESC"
	if order == "asc" { orderDir = "ASC" }
	switch sortBy {
	case "rzye":
		orderCol = "rzye"
	case "rqye":
		orderCol = "rqye"
	}
	query := fmt.Sprintf(`
		SELECT k.code, sb.name,
			COALESCE((k.buy_vol - k.sell_vol) * k.close, 0)/1e8 as net_flow,
			COALESCE(m.rzye, 0)/1e8 as rzye,
			COALESCE(m.rqye, 0)/1e8 as rqye,
			k.trade_date::text
		FROM stocks_daily_k k
		JOIN stocks_basic sb ON sb.code = k.code
		LEFT JOIN LATERAL (SELECT rzye, rqye FROM margin_trading WHERE code = k.code ORDER BY trade_date DESC LIMIT 1) m ON true
		WHERE k.trade_date = (SELECT MAX(trade_date) FROM stocks_daily_k WHERE buy_vol > 0)
		ORDER BY %s %s NULLS LAST LIMIT ?
	`, orderCol, orderDir)
	var list []StockCapitalRank
	db.PG.Raw(query, limit).Scan(&list)
	return list, nil
}
