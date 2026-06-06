package model

import "time"

// StockNews stores news, announcements, and research reports per stock
type StockNews struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	Code            string    `gorm:"index:idx_news_code;size:10" json:"code"`
	Title           string    `gorm:"size:500" json:"title"`
	Summary         string    `gorm:"type:text" json:"summary"`
	Source          string    `gorm:"size:50" json:"source"`       // eastmoney/cninfo/ths
	NewsType        string    `gorm:"size:20" json:"newsType"`     // news/announcement/report
	Url             string    `gorm:"size:500" json:"url"`
	PublishDate     string    `gorm:"index;size:10" json:"publishDate"`
	CreatedAt       time.Time `json:"createdAt"`
}

func (StockNews) TableName() string { return "stock_news" }
