package handlers

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/novasphere/novasphere/internal/models"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type MonitoringHandler struct {
	Native        *NativeHandler
	ClusterHealth func(context.Context) (string, error)
	DB            *gorm.DB
	Log           *logrus.Logger
}

func NewMonitoringHandler(db *gorm.DB, log *logrus.Logger) *MonitoringHandler {
	return &MonitoringHandler{DB: db, Log: log}
}

// Inventory counts are observed database state. Telemetry remains unavailable
// until a verified collector is configured; never substitute sample values.
func (h *MonitoringHandler) GetDashboardMetrics(c *gin.Context) {
	counts := map[string]int64{}
	for _, item := range []struct{ name, status string }{
		{"total", ""}, {"running", "Running"}, {"stopped", "Stopped"},
		{"error", "Error"}, {"migrating", "Migrating"},
		{"paused", "Paused"}, {"provisioning", "Provisioning"},
	} {
		query := h.DB.WithContext(c.Request.Context()).Model(&models.VirtualMachine{})
		if item.status != "" {
			query = query.Where("status = ?", item.status)
		}
		var count int64
		if err := query.Count(&count).Error; err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Inventory unavailable"})
			return
		}
		counts[item.name] = count
	}
	var total, ready int64
	db := h.DB.WithContext(c.Request.Context())
	if err := db.Model(&models.Host{}).Count(&total).Error; err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Inventory unavailable"})
		return
	}
	if err := db.Model(&models.Host{}).Where("status = ?", "Ready").Count(&ready).Error; err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Inventory unavailable"})
		return
	}
	payload := gin.H{
		"vms":              counts,
		"hosts":            gin.H{"total": total, "ready": ready},
		"cluster_health":   "Unknown",
		"telemetry_status": "unavailable",
		"utilization":      gin.H{"cpu_percent": nil, "memory_percent": nil, "storage_percent": nil, "network_mbps": nil},
	}
	if h.Native != nil {
		for key, value := range h.Native.dashboardTelemetry(c.Request.Context()) {
			payload[key] = value
		}
	}
	if h.ClusterHealth != nil {
		health, err := h.ClusterHealth(c.Request.Context())
		if err == nil {
			payload["ceph_health"] = health
			payload["cluster_health"] = combinedClusterHealth(payload["host_health"], health)
		}
	}
	payload["cluster_health_scope"] = "Enrolled KVM hosts and Ceph storage"
	c.JSON(http.StatusOK, payload)
}

// Alert Rules
func (h *MonitoringHandler) ListAlertRules(c *gin.Context) {
	var rules []models.AlertRule
	if err := h.DB.WithContext(c.Request.Context()).Find(&rules).Error; err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Alert rules unavailable"})
		return
	}
	c.JSON(http.StatusOK, rules)
}

func (h *MonitoringHandler) CreateAlertRule(c *gin.Context) {
	var rule models.AlertRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.DB.Create(&rule).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create alert rule"})
		return
	}
	c.JSON(http.StatusCreated, rule)
}

func (h *MonitoringHandler) UpdateAlertRule(c *gin.Context) {
	id, _ := uuid.Parse(c.Param("id"))
	var rule models.AlertRule
	if err := h.DB.First(&rule, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Alert rule not found"})
		return
	}
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	h.DB.Save(&rule)
	c.JSON(http.StatusOK, rule)
}

func (h *MonitoringHandler) DeleteAlertRule(c *gin.Context) {
	id, _ := uuid.Parse(c.Param("id"))
	h.DB.Delete(&models.AlertRule{}, "id = ?", id)
	c.Status(http.StatusNoContent)
}

// Events feed
func (h *MonitoringHandler) GetEvents(c *gin.Context) {
	var logs []models.AuditLog
	query := h.DB.WithContext(c.Request.Context()).Model(&models.AuditLog{}).Order("timestamp DESC")

	if resource := c.Query("resource"); resource != "" {
		query = query.Where("resource ILIKE ?", "%"+resource+"%")
	}

	page, perPage := parsePagination(c)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Audit events unavailable"})
		return
	}
	if err := query.Offset((page - 1) * perPage).Limit(perPage).Find(&logs).Error; err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Audit events unavailable"})
		return
	}

	c.JSON(http.StatusOK, PaginatedResponse{Data: logs, Total: total, Page: page, PerPage: perPage})
}

func combinedClusterHealth(host any, ceph string) string {
	if host == "Degraded" || ceph == "HEALTH_ERR" {
		return "Degraded"
	}
	if ceph == "HEALTH_WARN" {
		return "Warning"
	}
	if host == "Healthy" && ceph == "HEALTH_OK" {
		return "Healthy"
	}
	return "Unknown"
}
