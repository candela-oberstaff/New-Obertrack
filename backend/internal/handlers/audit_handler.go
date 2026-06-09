package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/obertrack/backend/internal/service"
)

type AuditHandler struct {
	svc service.AuditService
}

func NewAuditHandler(svc service.AuditService) *AuditHandler {
	return &AuditHandler{svc: svc}
}

// GetLogs returns a paginated, filterable list of audit entries (superadmin only).
func (h *AuditHandler) GetLogs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "25"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 200 {
		limit = 25
	}
	offset := (page - 1) * limit

	filters := map[string]interface{}{}
	if v := c.Query("email"); v != "" {
		filters["email"] = v
	}
	if v := c.Query("module"); v != "" {
		filters["module"] = v
	}
	if v := c.Query("kind"); v == "activity" || v == "data" {
		filters["kind"] = v
	}
	if v := c.Query("entity_type"); v != "" {
		filters["entity_type"] = v
	}
	if v := c.Query("entity_id"); v != "" {
		filters["entity_id"] = v
	}
	if v := c.Query("action"); v != "" {
		filters["action"] = v
	}
	if v := c.Query("q"); v != "" {
		filters["q"] = v
	}
	if v := c.Query("success"); v == "true" || v == "false" {
		filters["success"] = v == "true"
	}
	if v := c.Query("start_date"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			filters["start_date"] = t
		}
	}
	if v := c.Query("end_date"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			// include the whole end day
			filters["end_date"] = t.Add(24*time.Hour - time.Nanosecond)
		}
	}

	logs, total, err := h.svc.List(filters, offset, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch audit logs"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": logs, "total": total, "page": page, "limit": limit})
}
