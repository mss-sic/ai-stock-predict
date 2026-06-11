package handler

import (
	"fmt"
	"log"
	"strconv"

	"github.com/ai-stock-predict/server/internal/db"
	"github.com/ai-stock-predict/server/internal/model"
	"github.com/ai-stock-predict/server/pkg/response"
	"github.com/gin-gonic/gin"
)

type HoldingHandler struct{}

func NewHoldingHandler() *HoldingHandler { return &HoldingHandler{} }

type HoldingOut struct {
	ID        uint    `json:"id"`
	StockCode string  `json:"stockCode"`
	StockName string  `json:"stockName"`
	CostPrice float64 `json:"costPrice"`
	Quantity  int     `json:"quantity"`
	CurPrice  float64 `json:"curPrice"`
	MarketVal float64 `json:"marketVal"`
	Pnl       float64 `json:"pnl"`
	PnlPct    float64 `json:"pnlPct"`
}

// List returns user's holdings enriched with latest price from PG
func (h *HoldingHandler) List(c *gin.Context) {
	uid := getUID(c)
	var holdings []model.Holding
	db.MySQL.Where("user_id = ?", uid).Order("created_at DESC").Find(&holdings)

	if len(holdings) == 0 {
		response.Success(c, []HoldingOut{})
		return
	}

	codes := make([]string, len(holdings))
	for i, h := range holdings {
		codes[i] = h.StockCode
	}

	// Fetch latest close & name from PG
	type PriceInfo struct {
		Code  string
		Name  string
		Close float64
	}
	var infos []PriceInfo
	infoMap := make(map[string]PriceInfo)
	if err := db.PG.Raw(fmt.Sprintf(`SELECT s.code, s.name, COALESCE(k.close, 0) AS close
		FROM stocks_basic s
		LEFT JOIN LATERAL (SELECT close FROM stocks_daily_k WHERE code = s.code ORDER BY trade_date DESC LIMIT 1) k ON true
		WHERE s.code IN (%s)`, db.CodesToInClause(codes))).Scan(&infos).Error; err != nil {
		log.Printf("[holding] price info query failed: %v", err)
	}
	for _, info := range infos {
		infoMap[info.Code] = info
	}

	out := make([]HoldingOut, 0, len(holdings))
	for _, h := range holdings {
		info := infoMap[h.StockCode]
		curPrice := info.Close
		marketVal := curPrice * float64(h.Quantity)
		pnl := (curPrice - h.CostPrice) * float64(h.Quantity)
		pnlPct := 0.0
		if h.CostPrice > 0 {
			pnlPct = (curPrice - h.CostPrice) / h.CostPrice * 100
		}
		out = append(out, HoldingOut{
			ID:        h.ID,
			StockCode: h.StockCode,
			StockName: info.Name,
			CostPrice: h.CostPrice,
			Quantity:  h.Quantity,
			CurPrice:  curPrice,
			MarketVal: marketVal,
			Pnl:       pnl,
			PnlPct:    pnlPct,
		})
	}
	response.Success(c, out)
}

// Create adds a new holding
func (h *HoldingHandler) Create(c *gin.Context) {
	uid := getUID(c)
	if uid == 0 {
		response.Unauthorized(c, "未登录")
		return
	}
	var body struct {
		StockCode string  `json:"stockCode"`
		CostPrice float64 `json:"costPrice"`
		Quantity  int     `json:"quantity"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.StockCode == "" || body.Quantity <= 0 || body.CostPrice <= 0 {
		response.BadRequest(c, "参数错误：stockCode/costPrice/quantity 必填且大于0")
		return
	}

	// Verify stock exists in PG
	var count int64
	if err := db.PG.Raw("SELECT COUNT(*) FROM stocks_basic WHERE code = ?", body.StockCode).Scan(&count).Error; err != nil {
		response.Error(c, 500, response.CodeInternalError, "查询股票代码失败: "+err.Error())
		return
	}
	if count == 0 {
		response.BadRequest(c, "股票代码不存在: "+body.StockCode)
		return
	}

	holding := model.Holding{
		UserID:    uid,
		StockCode: body.StockCode,
		CostPrice: body.CostPrice,
		Quantity:  body.Quantity,
	}
	if err := db.MySQL.Create(&holding).Error; err != nil {
		response.Error(c, 500, 5001, "创建持仓失败: "+err.Error())
		return
	}
	response.Created(c, holding)
}

// Update modifies cost & quantity
func (h *HoldingHandler) Update(c *gin.Context) {
	uid := getUID(c)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	var body struct {
		CostPrice float64 `json:"costPrice"`
		Quantity  int     `json:"quantity"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Quantity <= 0 || body.CostPrice <= 0 {
		response.BadRequest(c, "参数错误：costPrice/quantity 必填且大于0")
		return
	}

	result := db.MySQL.Model(&model.Holding{}).
		Where("id = ? AND user_id = ?", id, uid).
		Updates(map[string]interface{}{
			"cost_price": body.CostPrice,
			"quantity":   body.Quantity,
		})
	if result.RowsAffected == 0 {
		response.NotFound(c, "持仓记录不存在")
		return
	}
	response.SuccessMsg(c, "更新成功")
}

// Delete removes a holding
func (h *HoldingHandler) Delete(c *gin.Context) {
	uid := getUID(c)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	result := db.MySQL.Where("id = ? AND user_id = ?", id, uid).Delete(&model.Holding{})
	if result.RowsAffected == 0 {
		response.NotFound(c, "持仓记录不存在")
		return
	}
	response.SuccessMsg(c, "删除成功")
}
