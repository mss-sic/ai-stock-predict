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

func (r *StockRepo) GetIndustryReports(industry string, limit int) ([]model.StockReport, error) {
	var rows []model.StockReport
	err := db.PG.Where("industry_name = ?", industry).Order("publish_date DESC").Limit(limit).Find(&rows).Error
	return rows, err
}
