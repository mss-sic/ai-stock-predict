package model

import "time"

// TradingAccount represents a user's trading account (real brokerage or simulated).
// Fields align with mx-moni (东方财富模拟盘) data model for seamless sync.
type TradingAccount struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	UserID         uint      `gorm:"index" json:"userId"`
	Name           string    `gorm:"size:100" json:"name"`                  // 账户名称
	Broker         string    `gorm:"size:50" json:"broker"`                 // 券商名称
	AccountType    string    `gorm:"size:20;default:simulated" json:"accountType"` // real / simulated
	AccountNumber  string    `gorm:"size:50" json:"accountNumber"`           // 资金账号

	// ── 资金字段 (对齐 mx-moni / 东财模拟盘) ──
	InitialCapital   float64 `gorm:"type:numeric(16,2);default:0" json:"initialCapital"`   // 初始本金 (mx-moni: initMoney)
	AvailableCash    float64 `gorm:"type:numeric(16,2);default:0" json:"availableCash"`     // 可用余额 (mx-moni: availBalance)
	FrozenCash       float64 `gorm:"type:numeric(16,2);default:0" json:"frozenCash"`        // 冻结资金 (mx-moni: frozenMoney)
	TotalAssets      float64 `gorm:"type:numeric(16,2);default:0" json:"totalAssets"`       // 总资产 = 可用 + 持仓市值 (mx-moni: totalAssets)
	TotalMarketValue float64 `gorm:"type:numeric(16,2);default:0" json:"totalMarketValue"`  // 持仓总市值 (mx-moni: totalPosValue)
	TotalProfit      float64 `gorm:"type:numeric(16,2);default:0" json:"totalProfit"`       // 累计盈亏 (mx-moni: totalProfit)
	Nav              float64 `gorm:"type:numeric(10,4);default:1" json:"nav"`               // 净值 (mx-moni: nav)
	TotalDeposit     float64 `gorm:"type:numeric(16,2);default:0" json:"totalDeposit"`      // 累计入金
	TotalWithdraw    float64 `gorm:"type:numeric(16,2);default:0" json:"totalWithdraw"`     // 累计出金

	Status     string `gorm:"size:20;default:active" json:"status"`        // active / archived

	// ── 券商集成 ──
	MxAPIKey    string `gorm:"size:200" json:"mxApiKey"`                    // 妙想 API Key
	MxAccountID string `gorm:"size:50" json:"mxAccountId"`                  // 妙想账户 ID (mx-moni: accID)
	BrokerMode  string `gorm:"size:20;default:manual" json:"brokerMode"`    // 执行模式: manual / mx_moni / lobster

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (TradingAccount) TableName() string { return "trading_accounts" }
