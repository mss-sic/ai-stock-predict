package handler

import (
	"time"
	"strconv"

	"github.com/ai-stock-predict/server/internal/db"
	"github.com/ai-stock-predict/server/internal/model"
	"github.com/ai-stock-predict/server/pkg/response"
	"github.com/gin-gonic/gin"
)

type WatchlistHandler struct{}

func NewWatchlistHandler() *WatchlistHandler { return &WatchlistHandler{} }

func getUID(c *gin.Context) uint {
	uid, exists := c.Get("userId")
	if !exists {
		return 0
	}
	switch v := uid.(type) {
	case uint:
		return v
	case float64:
		return uint(v)
	case int:
		return uint(v)
	case int64:
		return uint(v)
	default:
		return 0
	}
}

// ── Group APIs ──

func (h *WatchlistHandler) ListGroups(c *gin.Context) {
	uid := getUID(c)
	var groups []model.WatchlistGroup
	db.MySQL.Where("user_id = ?", uid).Order("sort_order ASC, id ASC").Find(&groups)
	response.Success(c, groups)
}

func (h *WatchlistHandler) CreateGroup(c *gin.Context) {
	uid := getUID(c)
	var body struct{ Name string `json:"name"` }
	if err := c.ShouldBindJSON(&body); err != nil || body.Name == "" {
		response.BadRequest(c, "分组名称不能为空")
		return
	}
	var count int64
	db.MySQL.Model(&model.WatchlistGroup{}).Where("user_id = ?", uid).Count(&count)
	if count >= 20 {
		response.BadRequest(c, "分组最多20个")
		return
	}
	grp := model.WatchlistGroup{UserID: uid, Name: body.Name, SortOrder: int(count)}
	if err := db.MySQL.Create(&grp).Error; err != nil {
		response.Error(c, 500, 5002, "创建分组失败: "+err.Error())
		return
	}
	response.Created(c, grp)
}

func (h *WatchlistHandler) RenameGroup(c *gin.Context) {
	uid := getUID(c)
	gid, _ := strconv.Atoi(c.Param("id"))
	var body struct{ Name string `json:"name"` }
	if err := c.ShouldBindJSON(&body); err != nil || body.Name == "" {
		response.BadRequest(c, "名称不能为空")
		return
	}
	db.MySQL.Model(&model.WatchlistGroup{}).Where("id = ? AND user_id = ?", gid, uid).
		Update("name", body.Name)
	response.SuccessMsg(c, "ok")
}

func (h *WatchlistHandler) DeleteGroup(c *gin.Context) {
	uid := getUID(c)
	gid, _ := strconv.Atoi(c.Param("id"))
	db.MySQL.Model(&model.Watchlist{}).Where("group_id = ? AND user_id = ?", gid, uid).
		Update("group_id", 0)
	db.MySQL.Where("id = ? AND user_id = ?", gid, uid).Delete(&model.WatchlistGroup{})
	response.SuccessMsg(c, "ok")
}

func (h *WatchlistHandler) ReorderGroups(c *gin.Context) {
	uid := getUID(c)
	var body struct{ IDs []uint `json:"ids"` }
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	for i, id := range body.IDs {
		db.MySQL.Model(&model.WatchlistGroup{}).Where("id = ? AND user_id = ?", id, uid).
			Update("sort_order", i)
	}
	response.SuccessMsg(c, "ok")
}

func (h *WatchlistHandler) MoveStock(c *gin.Context) {
	uid := getUID(c)
	code := c.Param("code")
	var body struct{ GroupId uint `json:"groupId"` }
	c.ShouldBindJSON(&body)
	db.MySQL.Model(&model.Watchlist{}).
		Where("user_id = ? AND stock_code = ?", uid, code).
		Update("group_id", body.GroupId)
	response.SuccessMsg(c, "ok")
}

// ── Stock APIs ──

type WatchlistStock struct {
	StockCode  string  `json:"stockCode"`
	StockName  string  `json:"stockName"`
	Close      float64 `json:"close"`
	AddedPrice float64 `json:"addedPrice"`
	AddedAt    string  `json:"addedAt"`
	Yield      float64 `json:"yield"`
	GroupID    uint    `json:"groupId"`
}

func (h *WatchlistHandler) ListStocks(c *gin.Context) {
	uid := getUID(c)
	gidStr := c.Query("groupId")
	var wl []model.Watchlist
	q := db.MySQL.Where("user_id = ?", uid)
	if gidStr != "" {
		gid, _ := strconv.Atoi(gidStr)
		q = q.Where("group_id = ?", gid)
	}
	q.Order("added_at DESC").Find(&wl)

	codes := make([]string, 0, len(wl))
	for _, w := range wl {
		codes = append(codes, w.StockCode)
	}

	type StockInfo struct {
		Code  string
		Name  string
		Close float64
	}
	var infos []StockInfo
	infoMap := make(map[string]StockInfo)
	if len(codes) > 0 {
		db.PG.Raw(`SELECT s.code, s.name, COALESCE(k.close, 0) AS close
			FROM stocks_basic s
			LEFT JOIN LATERAL (SELECT close FROM stocks_daily_k WHERE code = s.code ORDER BY trade_date DESC LIMIT 1) k ON true
			WHERE s.code = ANY($1)`, codes).Scan(&infos)
		for _, info := range infos {
			infoMap[info.Code] = info
		}
	}

	out := make([]WatchlistStock, 0, len(wl))
	for _, w := range wl {
		info := infoMap[w.StockCode]
		addedAt := ""
		if !w.AddedAt.IsZero() {
			addedAt = w.AddedAt.Format("2006-01-02")
		}
		yield := 0.0
		if w.AddedPrice > 0 && info.Close > 0 {
			yield = (info.Close - w.AddedPrice) / w.AddedPrice * 100
		}
		out = append(out, WatchlistStock{
			StockCode:  w.StockCode,
			StockName:  info.Name,
			Close:      info.Close,
			AddedPrice: w.AddedPrice,
			AddedAt:    addedAt,
			Yield:      yield,
			GroupID:    w.GroupID,
		})
	}
	response.Success(c, out)
}

func (h *WatchlistHandler) Add(c *gin.Context) {
	uid := getUID(c)
	if uid == 0 {
		response.Unauthorized(c, "未登录")
		return
	}
	var body struct {
		StockCode  string  `json:"stockCode"`
		GroupID    uint    `json:"groupId"`
		AddedPrice float64 `json:"addedPrice"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.StockCode == "" {
		response.BadRequest(c, "参数错误")
		return
	}

	// Check if already in watchlist
	var existing model.Watchlist
	if err := db.MySQL.Where("user_id = ? AND stock_code = ?", uid, body.StockCode).First(&existing).Error; err == nil {
		if body.GroupID > 0 && existing.GroupID != body.GroupID {
			db.MySQL.Model(&existing).Update("group_id", body.GroupID)
		}
		if body.AddedPrice > 0 {
			db.MySQL.Model(&existing).Update("added_price", body.AddedPrice)
		}
		response.SuccessMsg(c, "已更新")
		return
	}

	wl := model.Watchlist{
		AddedAt:    time.Now(),
		UserID:     uid,
		StockCode:  body.StockCode,
		GroupID:    body.GroupID,
		AddedPrice: body.AddedPrice,
	}
	if err := db.MySQL.Create(&wl).Error; err != nil {
		response.Error(c, 500, 5001, "添加失败: "+err.Error())
		return
	}
	response.SuccessMsg(c, "已添加")
}

func (h *WatchlistHandler) Remove(c *gin.Context) {
	uid := getUID(c)
	code := c.Param("code")
	db.MySQL.Where("user_id = ? AND stock_code = ?", uid, code).Delete(&model.Watchlist{})
	response.SuccessMsg(c, "已移除")
}

func (h *WatchlistHandler) Clear(c *gin.Context) {
	uid := getUID(c)
	gidStr := c.Query("groupId")
	q := db.MySQL.Where("user_id = ?", uid)
	if gidStr != "" {
		gid, _ := strconv.Atoi(gidStr)
		q = q.Where("group_id = ?", gid)
	}
	q.Delete(&model.Watchlist{})
	response.SuccessMsg(c, "已清空")
}
