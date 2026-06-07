package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// ── Error codes ──
const (
	CodeSuccess            = 0
	CodeBadRequest         = 1001
	CodeUnauthorized       = 1002
	CodeForbidden          = 1003
	CodeNotFound           = 1004
	CodeConflict           = 1005
	CodeInternalError      = 2001
	CodeAIConfigMissing    = 3001 // AI 未配置
	CodeAIModelError       = 3002 // AI 模型调用失败
	CodeDataMissing        = 4001 // 数据缺失
	CodeCollectError       = 4002 // 采集失败
	CodeDuplicateOperation = 4003 // 重复操作
)

// ── Response helpers ──

// Success returns 200 with data + code
func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, gin.H{
		"code":    CodeSuccess,
		"message": "ok",
		"data":    data,
	})
}

// SuccessMsg returns 200 with a message string as data
func SuccessMsg(c *gin.Context, msg string) {
	c.JSON(http.StatusOK, gin.H{
		"code":    CodeSuccess,
		"message": "ok",
		"data":    msg,
	})
}

// Created returns 201 with data + code
func Created(c *gin.Context, data interface{}) {
	c.JSON(http.StatusCreated, gin.H{
		"code":    CodeSuccess,
		"message": "ok",
		"data":    data,
	})
}

// Error returns error response with code
func Error(c *gin.Context, httpStatus int, code int, message string) {
	c.AbortWithStatusJSON(httpStatus, gin.H{
		"code":    code,
		"message": message,
	})
}

// BadRequest returns 400
func BadRequest(c *gin.Context, message string) {
	Error(c, http.StatusBadRequest, CodeBadRequest, message)
}

// Unauthorized returns 401
func Unauthorized(c *gin.Context, message string) {
	Error(c, http.StatusUnauthorized, CodeUnauthorized, message)
}

// Forbidden returns 403
func Forbidden(c *gin.Context, message string) {
	Error(c, http.StatusForbidden, CodeForbidden, message)
}

// NotFound returns 404
func NotFound(c *gin.Context, message string) {
	Error(c, http.StatusNotFound, CodeNotFound, message)
}

// Conflict returns 409
func Conflict(c *gin.Context, message string) {
	Error(c, http.StatusConflict, CodeConflict, message)
}

// InternalError returns 500
func InternalError(c *gin.Context, message string) {
	Error(c, http.StatusInternalServerError, CodeInternalError, message)
}
