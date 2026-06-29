package repository

import (
	"fmt"
	"strings"
	"time"

	"github.com/ai-stock-predict/server/internal/db"
	"github.com/ai-stock-predict/server/internal/model"
)

type StockRepo struct{}

// StockListRow enriched stock list row with latest daily K data.
type StockListRow struct {
	Code         string  `json:"code"`
	Name         string  `json:"name"`
	Industry     string  `json:"industry"`
	BoardType    string  `json:"boardType"`
	IsST         bool    `json:"isST"`
	Close        float64 `json:"close"`
	ChgPct       float64 `json:"chgPct"`
	Volume       int64   `json:"volume"`
	Amount       float64 `json:"amount"`
	TurnoverRate float64 `json:"turnoverRate"`
	TradeDate    string  `json:"tradeDate"`
}

// AppearanceRow represents a stock's appearance frequency in daily top-N rankings.
type AppearanceRow struct {
	Code      string  `json:"code"`
	Name      string  `json:"name"`
	Industry  string  `json:"industry"`
	BoardType string  `json:"boardType"`
	Appear5d  int     `json:"appear5d"`
	Appear20d int     `json:"appear20d"`
	Close     float64 `json:"close"`
	ChgPct    float64 `json:"chgPct"`
}

// UnusualRow represents a stock with unusual activity.
type UnusualRow struct {
	StockListRow
	UnusualTypes []string `json:"unusualTypes"`
	Amplitude    float64  `json:"amplitude"`
	AvgVol20     int64    `json:"avgVol20"`
}

// MarketSnapshot holds aggregate market overview data.
type MarketSnapshot struct {
	TradeDate    string `json:"tradeDate"`
	UpCount      int    `json:"upCount"`
	DownCount    int    `json:"downCount"`
	FlatCount    int    `json:"flatCount"`
	TotalStocks  int    `json:"totalStocks"`
	LimitUpCount int    `json:"limitUpCount"`
	LimitDownCount int  `json:"limitDownCount"`
	Amount       float64 `json:"amount"`     // 亿元
	PrevAmount   float64 `json:"prevAmount"` // 上一交易日成交额 亿元
	Change       float64 `json:"change"`
	ChangePct    float64 `json:"changePct"`
	CompositeScore float64 `json:"compositeScore"`
	ShAmount     float64 `json:"shAmount"`   // 上证成交额 亿元
	SzAmount     float64 `json:"szAmount"`   // 深证成交额 亿元
	CyAmount     float64 `json:"cyAmount"`   // 创业板成交额 亿元
	KcAmount     float64 `json:"kcAmount"`   // 科创板成交额 亿元
	BjAmount     float64 `json:"bjAmount"`   // 北交所成交额 亿元
	ShUp         int     `json:"shUp"`
	ShDown       int     `json:"shDown"`
	ShFlat       int     `json:"shFlat"`
	SzUp         int     `json:"szUp"`
	SzDown       int     `json:"szDown"`
	SzFlat       int     `json:"szFlat"`
	CyUp         int     `json:"cyUp"`
	CyDown       int     `json:"cyDown"`
	CyFlat       int     `json:"cyFlat"`
}

func (r *StockRepo) List(industry, keyword, boardType, sortBy, sortDir string, offset, limit int) ([]StockListRow, int64, error) {
	var rows []StockListRow
	var total int64

	// Find the two most recent trading dates (one query)
	var latestDate, prevDate time.Time
	db.PG.Raw("SELECT MAX(trade_date) FROM stocks_daily_k WHERE code !~ '^IDX'").Scan(&latestDate)
	db.PG.Raw("SELECT MAX(trade_date) FROM stocks_daily_k WHERE code !~ '^IDX' AND trade_date < ?", latestDate).Scan(&prevDate)

	// Build WHERE clause for stocks_basic
	where := " WHERE sb.code !~ '^IDX'"
	countArgs := []interface{}{}

	if industry != "" {
		where += " AND sb.industry = ?"
		countArgs = append(countArgs, industry)
	}
	if keyword != "" {
		where += " AND (sb.code LIKE ? OR sb.name LIKE ?)"
		countArgs = append(countArgs, keyword+"%", "%"+keyword+"%")
	}
	if boardType != "" {
		switch boardType {
		case "main":
			where += " AND sb.board_type IN ('sh','sz')"
		case "cy":
			where += " AND sb.board_type = 'cy'"
		case "kc":
			where += " AND sb.board_type = 'kc'"
		case "bj":
			where += " AND sb.board_type = 'bj'"
		case "etf-bond":
			where += " AND sb.board_type IN ('etf','bond')"
		}
	}

	// Fast count — only on stocks_basic, no join needed
	countSQL := "SELECT COUNT(*) FROM stocks_basic sb" + where
	if err := db.PG.Raw(countSQL, countArgs...).Scan(&total).Error; err != nil {
		return nil, 0, err
	}

	// Sort
	orderBy := " ORDER BY sb.code ASC"
	switch sortBy {
	case "chgPct":
		orderBy = " ORDER BY COALESCE((k.close - NULLIF(p.close, 0)) / NULLIF(p.close, 0) * 100, 0)"
	case "volume":
		orderBy = " ORDER BY COALESCE(k.volume, 0)"
	case "amount":
		orderBy = " ORDER BY COALESCE(k.amount, 0)"
	case "turnoverRate":
		orderBy = " ORDER BY COALESCE(k.turnover_rate, 0)"
	}
	if strings.ToUpper(sortDir) == "ASC" {
		orderBy += " ASC"
	} else if sortBy != "" {
		orderBy += " DESC"
	}
	orderBy += " NULLS LAST, sb.code ASC"

	// Efficient: use fixed-date equality joins instead of DISTINCT ON
	// k = latest trading date, p = previous trading date (both fixed calendar dates)
	selectSQL := fmt.Sprintf(`SELECT sb.code, sb.name, COALESCE(sb.industry, '') AS industry, COALESCE(sb.board_type, '') AS board_type, COALESCE(sb.is_st, false) AS is_st,
			COALESCE(k.close, 0) AS close,
			COALESCE((k.close - NULLIF(p.close, 0)) / NULLIF(p.close, 0) * 100, 0) AS chg_pct,
			COALESCE(k.volume, 0) AS volume,
			COALESCE(k.amount, 0) AS amount,
			COALESCE(k.turnover_rate, 0) AS turnover_rate,
			TO_CHAR('%s'::date, 'YYYY-MM-DD') AS trade_date
		FROM stocks_basic sb
		LEFT JOIN stocks_daily_k k ON k.code = sb.code AND k.trade_date = ?::date
		LEFT JOIN stocks_daily_k p ON p.code = sb.code AND p.trade_date = ?::date
		%s %s LIMIT ? OFFSET ?`, latestDate.Format("2006-01-02"), where, orderBy)

	allArgs := append([]interface{}{latestDate, prevDate}, countArgs...)
	allArgs = append(allArgs, limit, offset)
	if err := db.PG.Raw(selectSQL, allArgs...).Scan(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// GetMarketSnapshot returns aggregate market overview for the latest trading day.
func (r *StockRepo) GetMarketSnapshot() (*MarketSnapshot, error) {
	var snap MarketSnapshot
	err := db.PG.Raw(`
		SELECT
			TO_CHAR(a.trade_date, 'YYYY-MM-DD') AS trade_date,
			COALESCE(a.up_count, 0) AS up_count,
			COALESCE(a.down_count, 0) AS down_count,
			COALESCE(a.total_stocks - a.up_count - a.down_count, 0) AS flat_count,
			COALESCE(a.total_stocks, 0) AS total_stocks,
			COALESCE(s.limit_up_count, 0) AS limit_up_count,
			COALESCE(s.limit_down_count, 0) AS limit_down_count,
			COALESCE((SELECT SUM(amount) FROM stocks_daily_k WHERE trade_date = a.trade_date AND code !~ '^IDX') / 1e8, 0) AS amount,
			COALESCE((SELECT SUM(amount) FROM stocks_daily_k WHERE trade_date = (SELECT MAX(trade_date) FROM market_daily_agg WHERE trade_date < a.trade_date) AND code !~ '^IDX') / 1e8, 0) AS prev_amount,
			COALESCE(s.composite_score, 0) AS composite_score
		FROM market_daily_agg a
		LEFT JOIN market_sentiment s ON s.trade_date = a.trade_date
		ORDER BY a.trade_date DESC LIMIT 1
	`).Scan(&snap).Error
	if err != nil {
		return nil, err
	}
	snap.Change = snap.Amount - snap.PrevAmount
	if snap.PrevAmount > 0 {
		snap.ChangePct = (snap.Amount - snap.PrevAmount) / snap.PrevAmount * 100
	}
	// Fetch per-board turnover amounts (aggregated from individual stocks)
	type boardAmt struct {
		BoardType string  `gorm:"column:board_type"`
		Amount    float64 `gorm:"column:amount"`
	}
	var bamts []boardAmt
	db.PG.Raw(`
		SELECT b.board_type, SUM(k.amount) / 1e8 AS amount
		FROM stocks_daily_k k
		JOIN stocks_basic b ON b.code = k.code
		WHERE k.trade_date = ?::date
		GROUP BY b.board_type
	`, snap.TradeDate).Scan(&bamts)
	for _, ba := range bamts {
		switch ba.BoardType {
		case "sh":
			snap.ShAmount += ba.Amount
		case "sz":
			snap.SzAmount += ba.Amount
		case "cy":
			snap.CyAmount += ba.Amount
			snap.SzAmount += ba.Amount // 深证 = 深主 + 创业
		case "kc":
			snap.KcAmount += ba.Amount
			snap.ShAmount += ba.Amount // 上证 = 沪主 + 科创
		case "bj":
			snap.BjAmount = ba.Amount
		}
	}

	// Fetch per-board up/down/flat counts (optimized with fixed-date joins)
	var prevDate time.Time
	db.PG.Raw("SELECT MAX(trade_date) FROM stocks_daily_k WHERE code !~ '^IDX' AND trade_date < ?::date", snap.TradeDate).Scan(&prevDate)
	type boardUD struct {
		BoardType string `gorm:"column:board_type"`
		Up        int    `gorm:"column:up_cnt"`
		Down      int    `gorm:"column:down_cnt"`
		Flat      int    `gorm:"column:flat_cnt"`
	}
	var buds []boardUD
	db.PG.Raw(`
		SELECT b.board_type,
			COUNT(*) FILTER (WHERE k.close > p.close) AS up_cnt,
			COUNT(*) FILTER (WHERE k.close < p.close) AS down_cnt,
			COUNT(*) FILTER (WHERE k.close = p.close OR p.close IS NULL) AS flat_cnt
		FROM stocks_daily_k k
		JOIN stocks_basic b ON b.code = k.code
		LEFT JOIN stocks_daily_k p ON p.code = k.code AND p.trade_date = ?::date
		WHERE k.trade_date = ?::date
		GROUP BY b.board_type
	`, prevDate, snap.TradeDate).Scan(&buds)
	for _, bd := range buds {
		switch bd.BoardType {
		case "sh":
			snap.ShUp += bd.Up; snap.ShDown += bd.Down; snap.ShFlat += bd.Flat
		case "sz":
			snap.SzUp += bd.Up; snap.SzDown += bd.Down; snap.SzFlat += bd.Flat
		case "cy":
			snap.CyUp = bd.Up; snap.CyDown = bd.Down; snap.CyFlat = bd.Flat
			snap.SzUp += bd.Up; snap.SzDown += bd.Down; snap.SzFlat += bd.Flat
		case "kc":
			snap.ShUp += bd.Up; snap.ShDown += bd.Down; snap.ShFlat += bd.Flat
		}
	}

	return &snap, nil
}

// GetRanking returns top stocks by the given sort field.
func (r *StockRepo) GetRanking(boardType, sortBy string, limit int, asc bool) ([]StockListRow, error) {
	var rows []StockListRow

	// Find the two most recent trading dates
	var latestDate, prevDate time.Time
	db.PG.Raw("SELECT MAX(trade_date) FROM stocks_daily_k WHERE code !~ '^IDX'").Scan(&latestDate)
	db.PG.Raw("SELECT MAX(trade_date) FROM stocks_daily_k WHERE code !~ '^IDX' AND trade_date < ?", latestDate).Scan(&prevDate)

	direction := "DESC"
	if asc {
		direction = "ASC"
	}

	where := " WHERE sb.code !~ '^IDX' AND sb.is_st = false"
	args := []interface{}{latestDate, prevDate}

	if boardType != "" {
		switch boardType {
		case "main":
			where += " AND sb.board_type IN ('sh','sz')"
		case "cy":
			where += " AND sb.board_type = 'cy'"
		case "kc":
			where += " AND sb.board_type = 'kc'"
		case "bj":
			where += " AND sb.board_type = 'bj'"
		case "etf-bond":
			where += " AND sb.board_type IN ('etf','bond')"
		}
	}

	orderExpr := "COALESCE((k.close - NULLIF(p.close, 0)) / NULLIF(p.close, 0) * 100, 0)"
	switch sortBy {
	case "chgPct":
		orderExpr = "COALESCE((k.close - NULLIF(p.close, 0)) / NULLIF(p.close, 0) * 100, 0)"
	case "volume":
		orderExpr = "COALESCE(k.volume, 0)"
	case "amount":
		orderExpr = "COALESCE(k.amount, 0)"
	case "turnoverRate":
		orderExpr = "COALESCE(k.turnover_rate, 0)"
	}

	query := fmt.Sprintf(`SELECT sb.code, sb.name, COALESCE(sb.industry, '') AS industry, COALESCE(sb.board_type, '') AS board_type, COALESCE(sb.is_st, false) AS is_st,
			COALESCE(k.close, 0) AS close,
			COALESCE((k.close - NULLIF(p.close, 0)) / NULLIF(p.close, 0) * 100, 0) AS chg_pct,
			COALESCE(k.volume, 0) AS volume,
			COALESCE(k.amount, 0) AS amount,
			COALESCE(k.turnover_rate, 0) AS turnover_rate,
			TO_CHAR('%s'::date, 'YYYY-MM-DD') AS trade_date
		FROM stocks_basic sb
		LEFT JOIN stocks_daily_k k ON k.code = sb.code AND k.trade_date = ?::date
		LEFT JOIN stocks_daily_k p ON p.code = sb.code AND p.trade_date = ?::date
		%s ORDER BY %s %s NULLS LAST LIMIT ?`, latestDate.Format("2006-01-02"), where, orderExpr, direction)

	args = append(args, limit)
	if err := db.PG.Raw(query, args...).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// GetUnusual returns stocks with unusual activity on the latest trading day.
func (r *StockRepo) GetUnusual(boardType string, limit int) ([]UnusualRow, error) {
	var rows []UnusualRow

	var latestDate, prevDate time.Time
	db.PG.Raw("SELECT MAX(trade_date) FROM stocks_daily_k WHERE code !~ '^IDX'").Scan(&latestDate)
	db.PG.Raw("SELECT MAX(trade_date) FROM stocks_daily_k WHERE code !~ '^IDX' AND trade_date < ?", latestDate).Scan(&prevDate)

	where := " WHERE sb.code !~ '^IDX' AND sb.is_st = false"
	args := []interface{}{latestDate, prevDate, prevDate}

	if boardType != "" {
		switch boardType {
		case "main":
			where += " AND sb.board_type IN ('sh','sz')"
		case "cy":
			where += " AND sb.board_type = 'cy'"
		case "kc":
			where += " AND sb.board_type = 'kc'"
		case "bj":
			where += " AND sb.board_type = 'bj'"
		case "etf-bond":
			where += " AND sb.board_type IN ('etf','bond')"
		}
	}

	query := fmt.Sprintf(`SELECT sb.code, sb.name, COALESCE(sb.industry, '') AS industry, COALESCE(sb.board_type, '') AS board_type, COALESCE(sb.is_st, false) AS is_st,
		COALESCE(k.close, 0) AS close,
		COALESCE((k.close - NULLIF(p.close, 0)) / NULLIF(p.close, 0) * 100, 0) AS chg_pct,
		COALESCE(k.volume, 0) AS volume,
		COALESCE(k.amount, 0) AS amount,
		COALESCE(k.turnover_rate, 0) AS turnover_rate,
		TO_CHAR(k.trade_date, 'YYYY-MM-DD') AS trade_date,
		COALESCE((k.high - k.low) / NULLIF(p.close, 0) * 100, 0) AS amplitude,
		COALESCE(avg20.avg_vol, 0) AS avg_vol20
		FROM stocks_basic sb
		LEFT JOIN stocks_daily_k k ON k.code = sb.code AND k.trade_date = ?::date
		LEFT JOIN stocks_daily_k p ON p.code = sb.code AND p.trade_date = ?::date
		LEFT JOIN LATERAL (
			SELECT AVG(t.volume)::BIGINT AS avg_vol FROM (
				SELECT volume FROM stocks_daily_k
				WHERE code = sb.code AND trade_date < ?::date
				ORDER BY trade_date DESC LIMIT 20
			) t
		) avg20 ON true
		%s
		ORDER BY ABS(COALESCE((k.close - NULLIF(p.close, 0)) / NULLIF(p.close, 0) * 100, 0)) DESC
		LIMIT ?`, where)

	args = append(args, limit)
	if err := db.PG.Raw(query, args...).Scan(&rows).Error; err != nil {
		return nil, err
	}

	// Attach unusual type labels
	for i := range rows {
		var flags []string
		chgAbs := rows[i].ChgPct
		if chgAbs < 0 {
			chgAbs = -chgAbs
		}

		// Volume surge: today vol > 20-day avg * 2
		if rows[i].AvgVol20 > 0 && rows[i].Volume > rows[i].AvgVol20*2 {
			flags = append(flags, "放量")
		}
		// Price surge: depends on board
		bt := rows[i].BoardType
		threshold := 7.0
		if bt == "kc" || bt == "cy" {
			threshold = 14.0
		} else if bt == "bj" {
			threshold = 27.0
		}
		if chgAbs > threshold {
			if rows[i].ChgPct > 0 {
				flags = append(flags, "急涨")
			} else {
				flags = append(flags, "急跌")
			}
		}
		// Amplitude surge
		if rows[i].Amplitude > 10 {
			flags = append(flags, "高振幅")
		}
		rows[i].UnusualTypes = flags
	}

	// Filter to rows that actually have unusual flags
	var filtered []UnusualRow
	for _, r := range rows {
		if len(r.UnusualTypes) > 0 {
			filtered = append(filtered, r)
		}
	}
	return filtered, nil
}

// GetBoardTypeCounts returns stock counts per board type.
func (r *StockRepo) GetBoardTypeCounts() (map[string]int64, error) {
	type row struct {
		BoardType string `gorm:"column:board_type"`
		Cnt       int64  `gorm:"column:cnt"`
	}
	var rows []row
	db.PG.Raw(`SELECT COALESCE(board_type, 'other') AS board_type, COUNT(*) AS cnt
		FROM stocks_basic WHERE code !~ '^IDX' GROUP BY board_type`).Scan(&rows)

	result := make(map[string]int64)
	for _, r := range rows {
		result[r.BoardType] = r.Cnt
	}
	return result, nil
}

func (r *StockRepo) GetByCode(code string) (*model.StockBasic, error) {
	var stock model.StockBasic
	err := db.PG.Where("code = ?", code).First(&stock).Error
	return &stock, err
}

func (r *StockRepo) GetKLine(code string, from, to time.Time) ([]model.StockDailyK, error) {
	var klines []model.StockDailyK
	q := db.PG.Where("code = ?", code)
	if !from.IsZero() {
		q = q.Where("trade_date >= ?", from)
	}
	if !to.IsZero() {
		q = q.Where("trade_date <= ?", to)
	}
	err := q.Order("trade_date ASC").Find(&klines).Error
	return klines, err
}

func (r *StockRepo) GetIndicator(code string, date time.Time) (*model.StockDailyIndicator, error) {
	var ind model.StockDailyIndicator
	err := db.PG.Where("code = ?", code).Order("CASE WHEN pe > 0 THEN 0 ELSE 1 END, trade_date DESC").First(&ind).Error
	return &ind, err
}

func (r *StockRepo) UpsertBasic(stock *model.StockBasic) error {
	return db.PG.Where("code = ?", stock.Code).Assign(stock).FirstOrCreate(stock).Error
}

func (r *StockRepo) UpsertDailyK(k *model.StockDailyK) error {
	return db.PG.Where("code = ? AND trade_date = ?", k.Code, k.TradeDate).Assign(k).FirstOrCreate(k).Error
}

func (r *StockRepo) GetSignal(code string) (*model.StockSignal, error) {
	var signal model.StockSignal
	err := db.PG.Where("code = ?", code).First(&signal).Error
	return &signal, err
}

// ── Financial / Shareholder / News ──

func (r *StockRepo) GetFinancials(code string) ([]model.StockFinancial, error) {
	var rows []model.StockFinancial
	err := db.PG.Where("code = ?", code).Order("report_date DESC").Limit(12).Find(&rows).Error
	return rows, err
}

func (r *StockRepo) GetShareholders(code string) ([]model.StockShareholder, error) {
	var rows []model.StockShareholder
	err := db.PG.Where("code = ?", code).Order("report_date DESC").Limit(12).Find(&rows).Error
	return rows, err
}

func (r *StockRepo) GetNews(code string, limit int) ([]model.StockNews, error) {
	var rows []model.StockNews
	err := db.PG.Where("code = ?", code).Order("publish_date DESC").Limit(limit).Find(&rows).Error
	return rows, err
}

func (r *StockRepo) GetReports(code string, limit int) ([]model.StockReport, error) {
	var rows []model.StockReport
	err := db.PG.Where("stock_code = ?", code).Order("publish_date DESC").Limit(limit).Find(&rows).Error
	return rows, err
}

// GetAppearanceStats returns stocks ranked by how often they appeared in top-N daily gainers.
// GetAppearanceStats returns stocks ranked by how often they appeared in algorithm picks.
func (r *StockRepo) GetAppearanceStats(topN, limit int) ([]AppearanceRow, error) {
	var rows []AppearanceRow
	// Count how many times each stock appeared in algorithm_pick_details in recent 5/20 trading days.
	query := fmt.Sprintf(`
		WITH recent_picks AS (
			SELECT DISTINCT pick_date FROM algorithm_pick_details
			ORDER BY pick_date DESC LIMIT 20
		),
		counts AS (
			SELECT apd.stock_code,
				COUNT(*) FILTER (WHERE apd.pick_date >= (SELECT pick_date FROM recent_picks ORDER BY pick_date DESC OFFSET 4 LIMIT 1)) as appear5d,
				COUNT(*) FILTER (WHERE apd.pick_date IN (SELECT pick_date FROM recent_picks)) as appear20d
			FROM algorithm_pick_details apd
			JOIN recent_picks rp ON apd.pick_date = rp.pick_date
			GROUP BY apd.stock_code
			HAVING COUNT(*) > 0
		),
		latest_k AS (
			SELECT DISTINCT ON (code) code, close,
				COALESCE((close - NULLIF(LAG(close) OVER (PARTITION BY code ORDER BY trade_date),0)) / NULLIF(LAG(close) OVER (PARTITION BY code ORDER BY trade_date),0) * 100, 0) as chg_pct
			FROM stocks_daily_k WHERE code !~ '^IDX'
			ORDER BY code, trade_date DESC
		)
		SELECT sb.code, sb.name, COALESCE(sb.industry,'') as industry,
			COALESCE(sb.board_type,'') as board_type,
			c.appear5d as appear5d, c.appear20d as appear20d,
			COALESCE(lk.close, 0) as close,
			COALESCE(lk.chg_pct, 0) as chg_pct
		FROM counts c
		JOIN stocks_basic sb ON sb.code = c.stock_code
		LEFT JOIN latest_k lk ON lk.code = c.stock_code
		WHERE sb.code !~ '^IDX' AND sb.is_st = false
		ORDER BY c.appear5d DESC, c.appear20d DESC
		LIMIT %d
	`, limit)
	if err := db.PG.Raw(query).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *StockRepo) GetIndustryReports(industry string, limit int) ([]model.StockReport, error) {
	var rows []model.StockReport
	err := db.PG.Where("industry_name = ?", industry).Order("publish_date DESC").Limit(limit).Find(&rows).Error
	return rows, err
}

// GetDragonTigerList returns dragon tiger board appearances for a stock.
func (r *StockRepo) GetDragonTigerList(code string) ([]model.DragonTigerList, error) {
	var rows []model.DragonTigerList
	err := db.PG.Where("code = ?", code).Order("trade_date DESC").Limit(30).Find(&rows).Error
	return rows, err
}

// GetDragonTigerDetail returns seat-level detail for a specific date.
func (r *StockRepo) GetDragonTigerDetail(code, tradeDate string) ([]model.DragonTigerDetail, error) {
	var rows []model.DragonTigerDetail
	err := db.PG.Where("code = ? AND trade_date = ?", code, tradeDate).Order("net_amt DESC").Find(&rows).Error
	return rows, err
}

// GetBlockTrades returns block trade history for a stock.
func (r *StockRepo) GetBlockTrades(code string) ([]model.BlockTrade, error) {
	var rows []model.BlockTrade
	err := db.PG.Where("code = ?", code).Order("trade_date DESC").Limit(30).Find(&rows).Error
	return rows, err
}

// GetCninfoAnnouncements returns announcements for a stock.
func (r *StockRepo) GetCninfoAnnouncements(code string, limit int) ([]model.CninfoAnnouncement, error) {
	var rows []model.CninfoAnnouncement
	err := db.PG.Where("code = ?", code).Order("ann_date DESC").Limit(limit).Find(&rows).Error
	return rows, err
}

// GetRestrictedUnlocks returns upcoming unlocks for a stock.
func (r *StockRepo) GetRestrictedUnlocks(code string) ([]model.RestrictedShareUnlock, error) {
	var rows []model.RestrictedShareUnlock
	err := db.PG.Where("code = ? AND free_date >= CURRENT_DATE", code).Order("free_date ASC").Find(&rows).Error
	return rows, err
}

// GetAllFutureUnlocks returns all future unlocks across all stocks with stock names.
// GetThsHotConceptStats returns concept tag aggregation from THS hot stocks for a date range.
// GetThsEpsForecast returns consensus EPS forecast for a stock.
// GetAllAnnouncements returns all announcements with optional limit.
func (r *StockRepo) GetStockFundFlow(code string) ([]model.StockFundFlow, error) {
	var rows []model.StockFundFlow
	err := db.PG.Where("code = ?", code).Order("trade_date DESC").Limit(60).Find(&rows).Error
	return rows, err
}

func (r *StockRepo) GetAllAnnouncements(limit int) ([]model.CninfoAnnouncement, error) {
	if limit <= 0 { limit = 200 }
	var rows []model.CninfoAnnouncement
	err := db.PG.Order("ann_date DESC").Limit(limit).Find(&rows).Error
	return rows, err
}

func (r *StockRepo) GetThsEpsForecast(code string) ([]model.ThsEpsForecast, error) {
	var rows []model.ThsEpsForecast
	err := db.PG.Where("code = ?", code).Order("year ASC").Find(&rows).Error
	return rows, err
}

// GetMacroNews returns latest macro news with optional category filter.
func (r *StockRepo) GetMacroNews(category string, limit int) ([]model.MacroNews, error) {
	if limit <= 0 { limit = 50 }
	var rows []model.MacroNews
	q := db.PG.Order("news_time DESC").Limit(limit)
	if category != "" && category != "all" {
		q = q.Where("category = ?", category)
	}
	err := q.Find(&rows).Error
	return rows, err
}

// GetMacroCategories returns distinct macro news categories.
func (r *StockRepo) GetMacroCategories() ([]string, error) {
	var cats []string
	err := db.PG.Table("macro_news").Select("DISTINCT category").Where("category != ''").Pluck("category", &cats).Error
	return cats, err
}

func (r *StockRepo) GetThsHotConceptStats(days int) ([]map[string]interface{}, error) {
	if days <= 0 {
		days = 7
	}
	startDate := time.Now().AddDate(0, 0, -days).Format("2006-01-02")
	var rows []map[string]interface{}
	err := db.PG.Raw(`
		SELECT tag, COUNT(DISTINCT code) AS stock_count, COUNT(*) AS appear_count
		FROM (
			SELECT code, unnest(string_to_array(reason_tags, '+')) AS tag
			FROM ths_hot_stocks
			WHERE trade_date >= ?
		) t
		WHERE tag != ''
		GROUP BY tag
		ORDER BY stock_count DESC, appear_count DESC
		LIMIT 50
	`, startDate).Scan(&rows).Error
	return rows, err
}

func (r *StockRepo) GetAllFutureUnlocks(days int) ([]model.RestrictedShareUnlock, error) {
	if days <= 0 {
		days = 90
	}
	var rows []model.RestrictedShareUnlock
	endDate := time.Now().AddDate(0, 0, days).Format("2006-01-02")
	err := db.PG.Raw(`
		SELECT u.id, u.code, COALESCE(s.name, u.code) AS name, u.free_date, u.stock_type, u.shares, u.ratio, u.is_history, u.created_at
		FROM restricted_share_unlock u
		LEFT JOIN stocks_basic s ON u.code = s.code
		WHERE u.free_date >= CURRENT_DATE AND u.free_date <= ?
		ORDER BY u.free_date ASC, u.ratio DESC
	`, endDate).Scan(&rows).Error
	return rows, err
}

// GetDailyDragonTigerList returns full dragon tiger list for a given date.
func (r *StockRepo) GetDailyDragonTigerList(tradeDate string) ([]model.DragonTigerList, error) {
	var rows []model.DragonTigerList
	err := db.PG.Where("trade_date = ?", tradeDate).Order("net_buy_amt DESC").Find(&rows).Error
	return rows, err
}

// GetDragonTigerSeats returns seat detail for a stock on a given date.
func (r *StockRepo) GetDragonTigerSeats(code, tradeDate string) ([]model.DragonTigerDetail, error) {
	var rows []model.DragonTigerDetail
	err := db.PG.Where("code = ? AND trade_date = ?", code, tradeDate).Order("net_amt DESC").Find(&rows).Error
	return rows, err
}
