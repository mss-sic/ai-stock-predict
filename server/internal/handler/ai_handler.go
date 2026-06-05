package handler

import (
	"fmt"
	"math/rand"
	"net/http"

	"github.com/gin-gonic/gin"
)

type AIHandler struct{}

func NewAIHandler() *AIHandler { return &AIHandler{} }

func (h *AIHandler) Analyze(c *gin.Context) {
	var body struct {
		Code     string `json:"code"`
		Question string `json:"question"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}

	templates := []string{
		fmt.Sprintf("根据技术面分析，%s 目前处于震荡整理阶段，MACD指标显示多头动能温和增强。建议关注成交量是否配合突破，严格设置止损。", body.Code),
		fmt.Sprintf("%s 基本面稳健，近期北向资金持续净流入。行业景气度处于上行周期，中长期配置价值较为突出。", body.Code),
		fmt.Sprintf("从估值角度看，%s 当前市盈率处于历史中位水平。技术形态呈现底部抬升格局，短线有望挑战前期高点。", body.Code),
	}
	reply := templates[rand.Intn(len(templates))]
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"reply": reply, "code": body.Code}})
}
