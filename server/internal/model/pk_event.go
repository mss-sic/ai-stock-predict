package model

import "time"

// PkEvent is a strategy PK competition event.
type PkEvent struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	Name            string    `gorm:"size:100" json:"name"`
	Description     string    `gorm:"size:500" json:"description"`
	Type            string    `gorm:"size:10;default:backtest" json:"type"` // backtest / live
	InitialCapital  float64   `gorm:"type:numeric(16,2);default:100000" json:"initialCapital"`
	StartDate       time.Time `gorm:"type:date" json:"startDate"`
	EndDate         time.Time `gorm:"type:date" json:"endDate"`
	Status          string    `gorm:"size:15;default:draft" json:"status"` // draft / enrolling / running / completed
	StockPool       string    `gorm:"size:30" json:"stockPool"`
	StockPoolParams string    `gorm:"type:json;default:'[]'" json:"stockPoolParams"`
	MaxEntries      int       `gorm:"default:0" json:"maxEntries"` // 0=unlimited
	BannerText      string    `gorm:"size:200" json:"bannerText"`
	CreatedBy       uint      `json:"createdBy"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

func (PkEvent) TableName() string { return "pk_events" }

// PkEntry is a user's enrollment in a PK event.
type PkEntry struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	EventID     uint       `gorm:"index" json:"eventId"`
	UserID      uint       `gorm:"index" json:"userId"`
	StrategyID  uint       `json:"strategyId"`
	Status      string     `gorm:"size:15;default:pending" json:"status"` // pending / running / completed
	ResultID    *uint      `json:"resultId"`
	TotalReturn float64    `gorm:"type:numeric(10,4);default:0" json:"totalReturn"`
	SharpeRatio float64    `gorm:"type:numeric(8,4);default:0" json:"sharpeRatio"`
	MaxDrawdown float64    `gorm:"type:numeric(10,4);default:0" json:"maxDrawdown"`
	WinRate     float64    `gorm:"type:numeric(8,4);default:0" json:"winRate"`
	TradeCount  int        `gorm:"default:0" json:"tradeCount"`
	FinalEquity float64    `gorm:"type:numeric(16,2);default:0" json:"finalEquity"`
	FinalRank   int        `gorm:"default:0" json:"finalRank"`
	JoinedAt    time.Time  `json:"joinedAt"`
	CompletedAt *time.Time `json:"completedAt"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`

	// Joined fields
	StrategyName string `gorm:"-" json:"strategyName"`
	Username     string `gorm:"-" json:"username"`
}

func (PkEntry) TableName() string { return "pk_entries" }

// PkDailyRanking stores daily ranking snapshot for live PK.
type PkDailyRanking struct {
	ID               uint      `gorm:"primaryKey" json:"id"`
	EventID          uint      `gorm:"index" json:"eventId"`
	EntryID          uint      `json:"entryId"`
	Date             string    `gorm:"size:10" json:"date"`
	Equity           float64   `gorm:"type:numeric(16,2)" json:"equity"`
	DailyReturn      float64   `gorm:"type:numeric(10,4)" json:"dailyReturn"`
	CumulativeReturn float64   `gorm:"type:numeric(10,4)" json:"cumulativeReturn"`
	PositionsJSON    string    `gorm:"type:text" json:"positionsJson"`
	Rank             int       `gorm:"default:0" json:"rank"`
	TradeCount       int       `gorm:"default:0" json:"tradeCount"`
	CreatedAt        time.Time `json:"createdAt"`
}

func (PkDailyRanking) TableName() string { return "pk_daily_rankings" }
