package scheduler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Handler provides HTTP endpoints for scheduler management and observability.
type Handler struct {
	sched *UnifiedScheduler
}

// NewHandler creates a new scheduler HTTP handler.
func NewHandler(sched *UnifiedScheduler) *Handler {
	return &Handler{sched: sched}
}

// RegisterRoutes registers all scheduler endpoints on the given Gin group.
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	r.GET("/health", h.Health)
	r.GET("/tasks", h.ListInstances)
	r.POST("/tasks/:id/trigger", h.TriggerInstance)
	r.GET("/definitions", h.ListDefinitions)
	r.GET("/queues", h.QueueStatus)
	r.GET("/alerts", h.ListAlerts)
	r.DELETE("/alerts", h.ClearAlerts)
	r.GET("/history", h.TaskHistory)
	r.GET("/readiness", h.Readiness)
}

// Health returns the scheduler's health status.
func (h *Handler) Health(c *gin.Context) {
	health := h.sched.Health()
	c.JSON(http.StatusOK, health)
}

// ListInstances returns all task instances, optionally filtered.
func (h *Handler) ListInstances(c *gin.Context) {
	defID := c.Query("definitionId")
	ownerKind := c.Query("ownerKind")
	var owner *ResourceRef
	if ownerKind != "" {
		ownerID, _ := strconv.Atoi(c.Query("ownerId"))
		owner = &ResourceRef{Kind: ownerKind, ID: uint(ownerID)}
	}
	instances := h.sched.ListInstances(defID, owner)
	c.JSON(http.StatusOK, instances)
}

// TriggerInstance manually triggers a task instance.
func (h *Handler) TriggerInstance(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid instance id"})
		return
	}
	if err := h.sched.TriggerNow(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "triggered", "instanceId": id})
}

// ListDefinitions returns all registered task definitions.
func (h *Handler) ListDefinitions(c *gin.Context) {
	kind := TaskKind(c.Query("kind"))
	defs := h.sched.ListDefinitions(kind)
	c.JSON(http.StatusOK, defs)
}

// QueueStatus returns per-kind queue depth and timing metrics.
func (h *Handler) QueueStatus(c *gin.Context) {
	stats := h.sched.queue.Stats()
	c.JSON(http.StatusOK, stats)
}

// ListAlerts returns active health alerts.
func (h *Handler) ListAlerts(c *gin.Context) {
	health := h.sched.Health()
	c.JSON(http.StatusOK, health.ActiveAlerts)
}

// ClearAlerts clears all active health alerts.
func (h *Handler) ClearAlerts(c *gin.Context) {
	h.sched.ClearAlerts()
	c.JSON(http.StatusOK, gin.H{"status": "cleared"})
}
// TaskHistory returns recent task execution records.
func (h *Handler) TaskHistory(c *gin.Context) {
	defID := c.Query("definitionId")
	limit := 50
	if l, err := strconv.Atoi(c.Query("limit")); err == nil && l > 0 && l <= 200 {
		limit = l
	}
	records := h.sched.GetTaskHistory(defID, limit)
	c.JSON(http.StatusOK, records)
}
// Readiness returns distributed execution readiness status.
func (h *Handler) Readiness(c *gin.Context) {
	status := h.sched.DistributedReadinessCheck()
	c.JSON(http.StatusOK, status)
}

