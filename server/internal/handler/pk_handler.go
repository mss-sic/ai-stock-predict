package handler

import (
	"context"
	"encoding/json"
	"log"
	"strconv"
	"time"

	"github.com/ai-stock-predict/server/internal/db"
	"github.com/ai-stock-predict/server/internal/model"
	"github.com/ai-stock-predict/server/pkg/response"
	"github.com/gin-gonic/gin"
)

type PkHandler struct{}

func NewPkHandler() *PkHandler { return &PkHandler{} }

// ── Create Event (admin) ──

type createPkEventReq struct {
	Name            string  `json:"name"`
	Description     string  `json:"description"`
	Type            string  `json:"type"`
	InitialCapital  float64 `json:"initialCapital"`
	StartDate       string  `json:"startDate"`
	EndDate         string  `json:"endDate"`
	StockPool       string  `json:"stockPool"`
	StockPoolParams string  `json:"stockPoolParams"`
	MaxEntries      int     `json:"maxEntries"`
	BannerText      string  `json:"bannerText"`
}

func (h *PkHandler) CreateEvent(c *gin.Context) {
	uid := getUID(c)
	var body createPkEventReq
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if body.Name == "" {
		response.BadRequest(c, "活动名称不能为空")
		return
	}
	if body.Type == "" {
		body.Type = "backtest"
	}
	if body.InitialCapital <= 0 {
		body.InitialCapital = 100000
	}

	startDate, _ := time.Parse("2006-01-02", body.StartDate)
	endDate, _ := time.Parse("2006-01-02", body.EndDate)

	event := model.PkEvent{
		Name:            body.Name,
		Description:     body.Description,
		Type:            body.Type,
		InitialCapital:  body.InitialCapital,
		StartDate:       startDate,
		EndDate:         endDate,
		Status:          "draft",
		StockPool:       body.StockPool,
		StockPoolParams: body.StockPoolParams,
		MaxEntries:      body.MaxEntries,
		BannerText:      body.BannerText,
		CreatedBy:       uid,
	}
	db.MySQL.Create(&event)
	response.Created(c, event)
}

// ── List Events ──

func (h *PkHandler) ListEvents(c *gin.Context) {
	status := c.Query("status")
	var events []model.PkEvent
	q := db.MySQL.Order("created_at DESC")
	if status != "" {
		q = q.Where("status = ?", status)
	}
	q.Find(&events)

	type eventOut struct {
		model.PkEvent
		EntryCount  int    `json:"entryCount"`
		CreatorName string `json:"creatorName"`
	}
	out := make([]eventOut, len(events))
	for i, e := range events {
		var count int64
		db.MySQL.Model(&model.PkEntry{}).Where("event_id = ?", e.ID).Count(&count)
		var uname string
		db.MySQL.Raw("SELECT COALESCE(username,'') FROM users WHERE id = ?", e.CreatedBy).Scan(&uname)
		out[i] = eventOut{
			PkEvent:     e,
			EntryCount:  int(count),
			CreatorName: uname,
		}
	}
	response.Success(c, out)
}

// ── Get Event Detail (with rankings) ──

func (h *PkHandler) GetEvent(c *gin.Context) {
	eid, _ := strconv.Atoi(c.Param("id"))
	var event model.PkEvent
	if db.MySQL.First(&event, eid).Error != nil {
		response.NotFound(c, "活动不存在")
		return
	}

	var entries []model.PkEntry
	db.MySQL.Where("event_id = ?", eid).Order("total_return DESC").Find(&entries)

	for i := range entries {
		var sname, uname string
		db.MySQL.Raw("SELECT COALESCE(name,'') FROM strategies WHERE id = ?", entries[i].StrategyID).Scan(&sname)
		db.MySQL.Raw("SELECT COALESCE(username,'') FROM users WHERE id = ?", entries[i].UserID).Scan(&uname)
		entries[i].StrategyName = sname
		entries[i].Username = uname
		entries[i].FinalRank = i + 1
	}

	var dailyRankings []model.PkDailyRanking
	if event.Type == "live" {
		db.MySQL.Where("event_id = ?", eid).Order("date, rank").Find(&dailyRankings)
	}

	response.Success(c, map[string]interface{}{
		"event":         event,
		"entries":       entries,
		"dailyRankings": dailyRankings,
	})
}

// ── Update Event (admin) ──

func (h *PkHandler) UpdateEvent(c *gin.Context) {
	eid, _ := strconv.Atoi(c.Param("id"))
	var event model.PkEvent
	if db.MySQL.First(&event, eid).Error != nil {
		response.NotFound(c, "活动不存在")
		return
	}
	var body map[string]interface{}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	updates := map[string]interface{}{}
	for _, k := range []string{"name", "description", "type", "stock_pool", "stock_pool_params", "banner_text"} {
		if v, ok := body[k]; ok {
			updates[k] = v
		}
	}
	if v, ok := body["initialCapital"]; ok {
		updates["initial_capital"] = v
	}
	if v, ok := body["maxEntries"]; ok {
		updates["max_entries"] = v
	}
	if v, ok := body["startDate"]; ok {
		if t, err := time.Parse("2006-01-02", v.(string)); err == nil {
			updates["start_date"] = t
		}
	}
	if v, ok := body["endDate"]; ok {
		if t, err := time.Parse("2006-01-02", v.(string)); err == nil {
			updates["end_date"] = t
		}
	}
	db.MySQL.Model(&event).Updates(updates)
	response.Success(c, event)
}

// ── Start Event ──

func (h *PkHandler) StartEvent(c *gin.Context) {
	eid, _ := strconv.Atoi(c.Param("id"))
	var event model.PkEvent
	if db.MySQL.First(&event, eid).Error != nil {
		response.NotFound(c, "活动不存在")
		return
	}
	if event.Status != "draft" {
		response.BadRequest(c, "只有草稿状态的活动可以开启")
		return
	}
	db.MySQL.Model(&event).Update("status", "enrolling")
	event.Status = "enrolling"
	response.Success(c, event)
}

// ── Close Event ──

func (h *PkHandler) CloseEvent(c *gin.Context) {
	eid, _ := strconv.Atoi(c.Param("id"))
	var event model.PkEvent
	if db.MySQL.First(&event, eid).Error != nil {
		response.NotFound(c, "活动不存在")
		return
	}
	db.MySQL.Model(&event).Update("status", "completed")
	event.Status = "completed"
	response.Success(c, event)
}

// ── Join Event ──

type joinPkReq struct {
	StrategyID uint `json:"strategyId"`
}

func (h *PkHandler) JoinEvent(c *gin.Context) {
	uid := getUID(c)
	eid, _ := strconv.Atoi(c.Param("id"))
	var body joinPkReq
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}

	var event model.PkEvent
	if db.MySQL.First(&event, eid).Error != nil {
		response.NotFound(c, "活动不存在")
		return
	}
	if event.Status != "enrolling" && event.Status != "running" {
		response.BadRequest(c, "活动当前不在报名阶段")
		return
	}

	if event.MaxEntries > 0 {
		var count int64
		db.MySQL.Model(&model.PkEntry{}).Where("event_id = ?", eid).Count(&count)
		if int(count) >= event.MaxEntries {
			response.BadRequest(c, "报名人数已满")
			return
		}
	}

	var existing model.PkEntry
	if db.MySQL.Where("event_id = ? AND user_id = ?", eid, uid).First(&existing).Error == nil {
		response.BadRequest(c, "您已报名该活动")
		return
	}

	var s model.Strategy
	if db.MySQL.Where("id = ? AND user_id = ?", body.StrategyID, uid).First(&s).Error != nil {
		response.BadRequest(c, "策略不存在")
		return
	}

	entry := model.PkEntry{
		EventID:    uint(eid),
		UserID:     uid,
		StrategyID: body.StrategyID,
		Status:     "pending",
		JoinedAt:   time.Now(),
	}
	db.MySQL.Create(&entry)

	if event.Type == "backtest" {
		go runPkBacktest(&event, &entry, &s)
	}

	response.Created(c, entry)
}

// ── Active Notice ──

func (h *PkHandler) ActiveNotice(c *gin.Context) {
	var events []model.PkEvent
	db.MySQL.Where("status IN ?", []string{"enrolling", "running"}).
		Order("created_at DESC").Limit(3).Find(&events)
	response.Success(c, events)
}

// ── Entry Detail ──

func (h *PkHandler) EntryDetail(c *gin.Context) {
	enid, _ := strconv.Atoi(c.Param("entryId"))
	var entry model.PkEntry
	if db.MySQL.First(&entry, enid).Error != nil {
		response.NotFound(c, "参赛记录不存在")
		return
	}

	if entry.ResultID == nil {
		response.Success(c, map[string]interface{}{"entry": entry, "result": nil})
		return
	}

	var result model.BacktestResult
	db.MySQL.First(&result, *entry.ResultID)

	var logs []model.BacktestExecutionLog
	db.MySQL.Where("task_id = ?", result.TaskID).Order("seq").Find(&logs)

	response.Success(c, map[string]interface{}{
		"entry":  entry,
		"result": result,
		"logs":   logs,
	})
}

// ── PK Backtest Runner ──

func runPkBacktest(event *model.PkEvent, entry *model.PkEntry, s *model.Strategy) {
	log.Printf("[pk] starting backtest for event=%d entry=%d strategy=%d", event.ID, entry.ID, s.ID)

	db.MySQL.Model(entry).Update("status", "running")

	stockCodes := parseStockPoolCodes(event.StockPool, event.StockPoolParams)

	startDate := event.StartDate.Format("2006-01-02")
	endDate := event.EndDate.Format("2006-01-02")

	paramsBytes, _ := json.Marshal(map[string]interface{}{
		"startDate":  startDate,
		"endDate":    endDate,
		"stockCodes": stockCodes,
		"stockPool":  event.StockPool,
		"pkEventId":  event.ID,
	})

	var totalDays int
	db.PG.Raw(`SELECT COUNT(DISTINCT trade_date) FROM stocks_daily_k 
		WHERE trade_date >= ? AND trade_date <= ?`, startDate, endDate).Scan(&totalDays)

	// Override strategy capital
	s.InitialCapital = event.InitialCapital

	task := model.BacktestTask{
		UserID:         entry.UserID,
		StrategyID:     entry.StrategyID,
		Status:         "pending",
		Phase:          "PK回测排队中",
		TotalDays:      totalDays,
		InitialCapital: event.InitialCapital,
		Params:         string(paramsBytes),
	}
	db.MySQL.Create(&task)

	// Register for cancellation
	rm := getRunningMap(s.ID)
	ctx, cancel := context.WithCancel(context.Background())
	rm[task.ID] = cancel

	// Run backtest
	defaultStrategyHandler.runBacktestAsync(ctx, &task, s, startDate, endDate, stockCodes)

	// After completion
	var updatedTask model.BacktestTask
	db.MySQL.First(&updatedTask, task.ID)
	if updatedTask.Status == "completed" && updatedTask.ResultID != nil {
		var result model.BacktestResult
		db.MySQL.First(&result, *updatedTask.ResultID)
		db.MySQL.Model(entry).Updates(map[string]interface{}{
			"status":       "completed",
			"result_id":    result.ID,
			"total_return": result.TotalReturn,
			"sharpe_ratio": result.SharpeRatio,
			"max_drawdown": result.MaxDrawdown,
			"win_rate":     result.WinRate,
			"trade_count":  result.TradeCount,
			"final_equity": result.FinalEquity,
			"completed_at": time.Now(),
		})
		rankPkEntries(entry.EventID)

		var pendingCount int64
		db.MySQL.Model(&model.PkEntry{}).
			Where("event_id = ? AND status IN ?", entry.EventID, []string{"pending", "running"}).
			Count(&pendingCount)
		if pendingCount == 0 {
			db.MySQL.Model(&model.PkEvent{}).Where("id = ?", entry.EventID).Update("status", "completed")
		}
	} else if updatedTask.Status == "failed" {
		db.MySQL.Model(entry).Updates(map[string]interface{}{
			"status":       "completed",
			"total_return": 0,
			"completed_at": time.Now(),
		})
		rankPkEntries(entry.EventID)
	}
}

func parseStockPoolCodes(pool string, params string) []string {
	if pool == "all" || pool == "" {
		return nil
	}
	var codes []string
	if params != "" {
		json.Unmarshal([]byte(params), &codes)
	}
	return codes
}

func rankPkEntries(eventID uint) {
	var entries []model.PkEntry
	db.MySQL.Where("event_id = ?", eventID).Order("total_return DESC").Find(&entries)
	for i, e := range entries {
		db.MySQL.Model(&e).Update("final_rank", i+1)
	}
}

// ── Live PK Daily Run ──

func RunLivePkDaily() {
	log.Println("[pk] running daily live PK...")
	var events []model.PkEvent
	db.MySQL.Where("type = ? AND status = ?", "live", "running").Find(&events)

	for _, event := range events {
		var entries []model.PkEntry
		db.MySQL.Where("event_id = ? AND status = ?", event.ID, "running").Find(&entries)

		today := time.Now().Format("2006-01-02")
		for _, entry := range entries {
			runLivePkDay(&event, &entry, today)
		}
		rankPkDaily(&event, today)
	}
	log.Printf("[pk] daily live PK complete for %d events", len(events))
}

func runLivePkDay(event *model.PkEvent, entry *model.PkEntry, date string) {
	var prevRanking model.PkDailyRanking
	prevEquity := event.InitialCapital
	prevCumRet := 0.0
	if err := db.MySQL.Where("entry_id = ?", entry.ID).Order("date DESC").First(&prevRanking).Error; err == nil {
		prevEquity = prevRanking.Equity
		prevCumRet = prevRanking.CumulativeReturn
	}

	ranking := model.PkDailyRanking{
		EventID:          event.ID,
		EntryID:          entry.ID,
		Date:             date,
		Equity:           prevEquity,
		DailyReturn:      0,
		CumulativeReturn: prevCumRet,
		TradeCount:       0,
	}
	db.MySQL.Where("entry_id = ? AND date = ?", entry.ID, date).
		Assign(ranking).FirstOrCreate(&ranking)

	db.MySQL.Model(entry).Updates(map[string]interface{}{
		"total_return": prevCumRet,
		"final_equity": prevEquity,
	})
}

func rankPkDaily(event *model.PkEvent, date string) {
	var rankings []model.PkDailyRanking
	db.MySQL.Where("event_id = ? AND date = ?", event.ID, date).
		Order("cumulative_return DESC").Find(&rankings)
	for i, r := range rankings {
		db.MySQL.Model(&r).Update("rank", i+1)
	}
}

// DefaultStrategyHandler is set by main, used by PK to delegate backtest execution.
var defaultStrategyHandler *StrategyHandler

func SetDefaultStrategyHandler(h *StrategyHandler) {
	defaultStrategyHandler = h
}
