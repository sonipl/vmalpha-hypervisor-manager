package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/novasphere/novasphere/internal/models"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type NetworkHandler struct {
	DB  *gorm.DB
	Log *logrus.Logger
}

func NewNetworkHandler(db *gorm.DB, log *logrus.Logger) *NetworkHandler {
	return &NetworkHandler{DB: db, Log: log}
}

func (h *NetworkHandler) ListNetworks(c *gin.Context) {
	var networks []models.Network
	query := h.DB.Model(&models.Network{})

	if ns := c.Query("namespace"); ns != "" {
		query = query.Where("namespace = ?", ns)
	}
	if netType := c.Query("type"); netType != "" {
		query = query.Where("type = ?", netType)
	}

	var total int64
	query.Count(&total)
	page, perPage := parsePagination(c)
	query.Offset((page - 1) * perPage).Limit(perPage).Find(&networks)

	c.JSON(http.StatusOK, PaginatedResponse{Data: networks, Total: total, Page: page, PerPage: perPage})
}

func (h *NetworkHandler) GetNetwork(c *gin.Context) {
	id, _ := uuid.Parse(c.Param("id"))
	var net models.Network
	if err := h.DB.First(&net, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Network not found"})
		return
	}
	c.JSON(http.StatusOK, net)
}

func (h *NetworkHandler) CreateNetwork(c *gin.Context) {
	var net models.Network
	if err := c.ShouldBindJSON(&net); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.DB.Create(&net).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create network"})
		return
	}
	c.JSON(http.StatusCreated, net)
}

func (h *NetworkHandler) UpdateNetwork(c *gin.Context) {
	id, _ := uuid.Parse(c.Param("id"))
	var net models.Network
	if err := h.DB.First(&net, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Network not found"})
		return
	}
	if err := c.ShouldBindJSON(&net); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	h.DB.Save(&net)
	c.JSON(http.StatusOK, net)
}

func (h *NetworkHandler) DeleteNetwork(c *gin.Context) {
	id, _ := uuid.Parse(c.Param("id"))
	h.DB.Delete(&models.Network{}, "id = ?", id)
	c.Status(http.StatusNoContent)
}

// Firewall Rules
func (h *NetworkHandler) ListFirewallRules(c *gin.Context) {
	var rules []models.FirewallRule
	query := h.DB.Model(&models.FirewallRule{})
	if ns := c.Query("namespace"); ns != "" {
		query = query.Where("namespace = ?", ns)
	}
	query.Order("priority ASC").Find(&rules)
	c.JSON(http.StatusOK, rules)
}

func (h *NetworkHandler) CreateFirewallRule(c *gin.Context) {
	var rule models.FirewallRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.DB.Create(&rule).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create rule"})
		return
	}
	c.JSON(http.StatusCreated, rule)
}

func (h *NetworkHandler) DeleteFirewallRule(c *gin.Context) {
	id, _ := uuid.Parse(c.Param("id"))
	h.DB.Delete(&models.FirewallRule{}, "id = ?", id)
	c.Status(http.StatusNoContent)
}
