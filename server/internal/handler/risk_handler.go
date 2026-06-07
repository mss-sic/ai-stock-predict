package handler

import (
	"log"
	"strconv"

	"github.com/ai-stock-predict/server/internal/model"
	"github.com/ai-stock-predict/server/internal/service"
	"github.com/ai-stock-predict/server/pkg/response"
	"github.com/gin-gonic/gin"
)

type RiskHandler struct{}

func NewRiskHandler() *RiskHandler { return &RiskHandler{} }

// List returns risk alerts for current user's holdings
func (h *RiskHandler) List(c *gin.Context) {
	uid := getUID(c)
	if uid == 0 {
		response.Unauthorized(c, "未登录")
		return
	}
	var alerts []model.RiskAlert
	alerts, err := service.GetUserRiskAlerts(uid)
	if err != nil {
		response.InternalError(c, "获取风险预警失败: "+err.Error())
		return
	}
	
	response.Success(c, alerts)
}

// Scan triggers a full risk scan (admin only via admin middleware)
func (h *RiskHandler) Scan(c *gin.Context) {
	count, err := service.ScanUserHoldings()
	if err != nil {
		response.InternalError(c, "风险扫描失败: "+err.Error())
		return
	}
	log.Printf("[RiskHandler] admin triggered scan: %d alerts generated", count)
	response.Success(c, map[string]any{"alertsGenerated": count})
}

// Ignore marks a specific alert as ignored
func (h *RiskHandler) Ignore(c *gin.Context) {
	uid := getUID(c)
	if uid == 0 {
		response.Unauthorized(c, "未登录")
		return
	}
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if err := service.IgnoreRiskAlert(uid, uint(id)); err != nil {
		response.InternalError(c, "忽略预警失败: "+err.Error())
		return
	}
	response.SuccessMsg(c, "已忽略")
}
