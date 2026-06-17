package handler

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/ai-stock-predict/server/internal/collector"
	"github.com/ai-stock-predict/server/internal/db"
	"github.com/ai-stock-predict/server/internal/model"
	"github.com/ai-stock-predict/server/pkg/response"
	"github.com/gin-gonic/gin"
)

type ImportHandler struct {
	aiH *AIHandler
}

func NewImportHandler(aiH *AIHandler) *ImportHandler { return &ImportHandler{aiH: aiH} }

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

func (h *ImportHandler) UploadKline(c *gin.Context) {
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

	result, err := parseKlineCSV(f, name)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	response.Success(c, result)
}

// ── Stock Profile Import ──

type profileEntry struct {
	StockCode       string `json:"stock_code"`
	RawCode         string `json:"raw_code"`
	CompanyName     string `json:"company_name"`
	RawName         string `json:"raw_name"`
	Market          string `json:"market"`
	AnalysisDate    string `json:"analysis_date"`
	AnalysisContent string `json:"analysis_content"`
}

func (h *ImportHandler) UploadProfile(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		response.BadRequest(c, "请选择JSON文件")
		return
	}

	f, err := file.Open()
	if err != nil {
		response.InternalError(c, "无法读取文件")
		return
	}
	defer f.Close()

	raw, err := io.ReadAll(f)
	if err != nil {
		response.InternalError(c, "读取文件失败")
		return
	}

	var entries []profileEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		response.BadRequest(c, "JSON格式错误: "+err.Error())
		return
	}

	if len(entries) == 0 {
		response.BadRequest(c, "文件无数据")
		return
	}

	imported := 0
	updated := 0
	errors := 0

	for _, e := range entries {
		code := strings.TrimSpace(e.RawCode)
		if code == "" && e.StockCode != "" {
			// Extract from "301176.SZ" → "301176"
			code = strings.Split(e.StockCode, ".")[0]
		}
		if len(code) < 6 {
			errors++
			continue
		}
		code = code[:6]

		// Parse analysis date, fallback to now
		analyzedAt := time.Now()
		if e.AnalysisDate != "" {
			if t, err := time.Parse("2006-01-02 15:04:05", e.AnalysisDate); err == nil {
				analyzedAt = t
			} else if t, err := time.Parse("2006-01-02", e.AnalysisDate); err == nil {
				analyzedAt = t
			}
		}

		profile := model.StockProfile{
			Code:            code,
			ProfileMarkdown: e.AnalysisContent,
			Source:          "import",
			AnalyzedAt:      analyzedAt,
		}

		// Upsert: if exists, update; otherwise create
		var existing model.StockProfile
		if err := db.PG.Where("code = ?", code).First(&existing).Error; err == nil {
			// Update existing
			db.PG.Model(&existing).Updates(map[string]interface{}{
				"profile_markdown": e.AnalysisContent,
				"source":           "import",
				"analyzed_at":      analyzedAt,
				"updated_at":       time.Now(),
			})
			updated++
		} else {
			// Create new
			db.PG.Create(&profile)
			imported++
		}
	}

	log.Printf("[profile-import] imported %d, updated %d, errors %d from %s", imported, updated, errors, file.Filename)
	response.Success(c, gin.H{
		"total":    len(entries),
		"imported": imported,
		"updated":  updated,
		"errors":   errors,
	})
}
