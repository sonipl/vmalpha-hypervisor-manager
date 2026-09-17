package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/novasphere/novasphere/internal/models"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type AIHandler struct {
	DB  *gorm.DB
	Log *logrus.Logger
}

func NewAIHandler(db *gorm.DB, log *logrus.Logger) *AIHandler {
	return &AIHandler{DB: db, Log: log}
}

// Recommendations
func (h *AIHandler) ListRecommendations(c *gin.Context) {
	var recs []models.AIRecommendation
	query := h.DB.Model(&models.AIRecommendation{})

	if recType := c.Query("type"); recType != "" {
		query = query.Where("type = ?", recType)
	}
	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}
	if severity := c.Query("severity"); severity != "" {
		query = query.Where("severity = ?", severity)
	}

	var total int64
	query.Count(&total)
	page, perPage := parsePagination(c)
	query.Order("created_at DESC").Offset((page - 1) * perPage).Limit(perPage).Find(&recs)

	c.JSON(http.StatusOK, PaginatedResponse{Data: recs, Total: total, Page: page, PerPage: perPage})
}

func (h *AIHandler) ApplyRecommendation(c *gin.Context) {
	id, _ := uuid.Parse(c.Param("id"))
	var rec models.AIRecommendation
	if err := h.DB.First(&rec, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Recommendation not found"})
		return
	}
	if rec.Status != "pending" {
		c.JSON(http.StatusConflict, gin.H{"error": "Recommendation already " + rec.Status})
		return
	}

	userID, _ := c.Get("user_id")
	uid := userID.(uuid.UUID)
	now := time.Now()

	h.DB.Model(&rec).Updates(map[string]interface{}{
		"status":     "applied",
		"applied_by": &uid,
		"applied_at": &now,
	})

	// In production: execute the action_json against KubeVirt/Ceph
	c.JSON(http.StatusOK, gin.H{"message": "Recommendation applied", "id": id})
}

func (h *AIHandler) DismissRecommendation(c *gin.Context) {
	id, _ := uuid.Parse(c.Param("id"))
	h.DB.Model(&models.AIRecommendation{}).Where("id = ?", id).Update("status", "dismissed")
	c.JSON(http.StatusOK, gin.H{"message": "Recommendation dismissed"})
}

// Automation Policies
func (h *AIHandler) ListPolicies(c *gin.Context) {
	var policies []models.AutomationPolicy
	h.DB.Find(&policies)
	c.JSON(http.StatusOK, policies)
}

func (h *AIHandler) CreatePolicy(c *gin.Context) {
	var policy models.AutomationPolicy
	if err := c.ShouldBindJSON(&policy); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.DB.Create(&policy).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create policy"})
		return
	}
	c.JSON(http.StatusCreated, policy)
}

func (h *AIHandler) UpdatePolicy(c *gin.Context) {
	id, _ := uuid.Parse(c.Param("id"))
	var policy models.AutomationPolicy
	if err := h.DB.First(&policy, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Policy not found"})
		return
	}

	var updates struct {
		Enabled   *bool `json:"enabled"`
		AutoApply *bool `json:"auto_apply"`
	}
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updateMap := map[string]interface{}{}
	if updates.Enabled != nil {
		updateMap["enabled"] = *updates.Enabled
	}
	if updates.AutoApply != nil {
		updateMap["auto_apply"] = *updates.AutoApply
	}

	h.DB.Model(&policy).Updates(updateMap)
	c.JSON(http.StatusOK, policy)
}

// AI Chat endpoint (NovaMind Assistant)
func (h *AIHandler) Chat(c *gin.Context) {
	var req struct {
		Message string `json:"message" binding:"required"`
		Context string `json:"context"` // current page context
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	username, _ := c.Get("username")
	role, _ := c.Get("role")

	// In production: forward to NovaMind ML service
	// For scaffold: return structured placeholder
	c.JSON(http.StatusOK, gin.H{
		"response": "NovaMind AI processing your request...",
		"intent":   "query",
		"user":     username,
		"role":     role,
		"actions":  []interface{}{},
		"sources":  []string{},
	})
}

// Dashboard AI Insights
func (h *AIHandler) GetInsights(c *gin.Context) {
	var recommendations []models.AIRecommendation
	h.DB.Where("status = ?", "pending").Order("severity DESC, confidence DESC").Limit(10).Find(&recommendations)

	// Capacity forecast stub
	insights := gin.H{
		"recommendations":      recommendations,
		"recommendation_count": len(recommendations),
		"capacity_forecast": gin.H{
			"cpu_days_until_80pct":     45,
			"memory_days_until_80pct":  62,
			"storage_days_until_80pct": 30,
		},
		"anomalies_24h": 3,
		"health_score":  87,
	}

	c.JSON(http.StatusOK, insights)
}

// Migration Assessment
func (h *AIHandler) AssessMigration(c *gin.Context) {
	var req struct {
		SourceType string   `json:"source_type" binding:"required"` // vmware, hyperv, olvm
		VMNames    []string `json:"vm_names" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// In production: AI model assesses each VM
	assessments := make([]gin.H, len(req.VMNames))
	for i, name := range req.VMNames {
		assessments[i] = gin.H{
			"vm_name":         name,
			"readiness_score": 0.92,
			"status":          "green",
			"blockers":        []string{},
			"warnings":        []string{"Check VirtIO driver compatibility"},
			"estimated_time":  "15 minutes",
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"source_type": req.SourceType,
		"assessments": assessments,
		"overall":     "Ready for migration",
	})
}
