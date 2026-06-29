package handler

import (
	"strconv"

	"github.com/ai-stock-predict/server/internal/service"
	"github.com/ai-stock-predict/server/pkg/response"
	"github.com/gin-gonic/gin"
)

type TaskHandler struct{}

func NewTaskHandler() *TaskHandler { return &TaskHandler{} }

func (h *TaskHandler) ListTasks(c *gin.Context) {
	tasks, err := service.ListTasks()
	if err != nil {
		response.InternalError(c, "获取任务列表失败")
		return
	}
	response.Success(c, tasks)
}

func (h *TaskHandler) CreateTask(c *gin.Context) {
	var body struct {
		Name     string `json:"name"`
		Phase    string `json:"phase"`
		CronExpr string `json:"cronExpr"`
		Enabled  bool   `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Name == "" || body.Phase == "" || body.CronExpr == "" {
		response.BadRequest(c, "参数错误：name/phase/cronExpr 必填")
		return
	}
	task, err := service.CreateTask(body.Name, body.Phase, body.CronExpr, body.Enabled)
	if err != nil {
		response.InternalError(c, "创建任务失败: "+err.Error())
		return
	}
	response.Created(c, task)
}

func (h *TaskHandler) UpdateTask(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	var body struct {
		Name     string `json:"name"`
		Phase    string `json:"phase"`
		CronExpr string `json:"cronExpr"`
		Enabled  *bool  `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	enabled := true
	if body.Enabled != nil {
		enabled = *body.Enabled
	}
	task, err := service.UpdateTask(uint(id), body.Name, body.Phase, body.CronExpr, enabled)
	if err != nil {
		response.InternalError(c, "更新任务失败: "+err.Error())
		return
	}
	response.Success(c, task)
}

func (h *TaskHandler) DeleteTask(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if err := service.DeleteTask(uint(id)); err != nil {
		response.InternalError(c, "删除任务失败")
		return
	}
	response.SuccessMsg(c, "已删除")
}

func (h *TaskHandler) RunTaskNow(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	var body struct {
		Args []string `json:"args"`
	}
	c.ShouldBindJSON(&body)
	if err := service.RunTaskNow(uint(id), body.Args); err != nil {
		response.InternalError(c, "执行任务失败: "+err.Error())
		return
	}
	response.SuccessMsg(c, "任务已触发")
}

func (h *TaskHandler) InitDefaults(c *gin.Context) {
	count, err := service.InitializeDefaultTasks()
	if err != nil {
		response.InternalError(c, "初始化失败: "+err.Error())
		return
	}
	response.Success(c, map[string]any{"created": count, "message": "默认任务初始化完成"})
}

func (h *TaskHandler) ResetTask(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil { response.BadRequest(c, "参数错误"); return }
	if err := service.ResetTaskStatus(uint(id)); err != nil {
		response.InternalError(c, "重置失败")
		return
	}
	response.SuccessMsg(c, "已重置")
}
func (h *TaskHandler) ToggleTask(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	task, err := service.ToggleTask(uint(id))
	if err != nil {
		response.InternalError(c, "操作失败: "+err.Error())
		return
	}
	response.Success(c, task)
}



func (h *TaskHandler) RepairTask(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	var body struct {
		From string `json:"from"`
		To   string `json:"to"`
		All  bool   `json:"all"`
	}
	c.ShouldBindJSON(&body)
	if err := service.RepairTask(uint(id), body.From, body.To, body.All); err != nil {
		response.InternalError(c, "修复任务失败: "+err.Error())
		return
	}
	response.SuccessMsg(c, "修复任务已触发")
}

func (h *TaskHandler) ListLogs(c *gin.Context) {
	taskID, _ := strconv.Atoi(c.Query("taskId"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	logs, err := service.ListTaskLogs(uint(taskID), limit)
	if err != nil {
		response.InternalError(c, "获取日志失败")
		return
	}
	response.Success(c, logs)
}
