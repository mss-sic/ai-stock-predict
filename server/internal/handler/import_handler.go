package handler

import (
	"net/http"

	"github.com/ai-stock-predict/server/internal/collector"
	"github.com/ai-stock-predict/server/internal/db"
	"github.com/ai-stock-predict/server/internal/model"
	"github.com/ai-stock-predict/server/pkg/response"
	"github.com/gin-gonic/gin"
)

type ImportHandler struct{}

func NewImportHandler() *ImportHandler { return &ImportHandler{} }

func (h *ImportHandler) Upload(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		response.BadRequest(c, "请选择文件")
		return
	}

	name := file.Filename
	if len(name) < 5 {
		response.BadRequest(c, "文件名无效")
		return
	}

	f, err := file.Open()
	if err != nil {
		response.InternalError(c, "无法读取文件")
		return
	}
	defer f.Close()

	result, err := collector.ParseAndImportExcel(f, name)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	response.Success(c, result)
}

func (h *ImportHandler) History(c *gin.Context) {
	var logs []model.ImportLog
	if db.MySQL != nil {
		db.MySQL.Order("imported_at DESC").Limit(20).Find(&logs)
	}
	response.Success(c, logs)
}
