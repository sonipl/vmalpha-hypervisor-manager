package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/novasphere/novasphere/internal/models"
	"github.com/novasphere/novasphere/internal/websocket"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type HostHandler struct {
	DB  *gorm.DB
	Hub *websocket.Hub
	Log *logrus.Logger
}

func NewHostHandler(db *gorm.DB, hub *websocket.Hub, log *logrus.Logger) *HostHandler {
	return &HostHandler{DB: db, Hub: hub, Log: log}
}

func (h *HostHandler) ListHosts(c *gin.Context) {
	sort, err := validatedSort(c.Query("sort"), c.Query("order"), hostSortFields)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	syncHostsFromKubernetes(h.DB, h.Log)

	var hosts []models.Host
	query := h.DB.Model(&models.Host{})

	if clusterID := c.Query("cluster_id"); clusterID != "" {
		query = query.Where("cluster_id = ?", clusterID)
	}
	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}
	if search := c.Query("search"); search != "" {
		query = query.Where("name ILIKE ?", "%"+search+"%")
	}

	var total int64
	query.Count(&total)

	page, perPage := parsePagination(c)
	query.Offset((page - 1) * perPage).Limit(perPage).
		Order(sort).
		Find(&hosts)

	c.JSON(http.StatusOK, PaginatedResponse{Data: hosts, Total: total, Page: page, PerPage: perPage})
}

func (h *HostHandler) GetHost(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid host ID"})
		return
	}

	var host models.Host
	if err := h.DB.First(&host, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Host not found"})
		return
	}
	c.JSON(http.StatusOK, host)
}

func (h *HostHandler) MaintenanceMode(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid host ID"})
		return
	}

	var host models.Host
	if err := h.DB.First(&host, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Host not found"})
		return
	}

	var req struct {
		Enable bool `json:"enable"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var newStatus models.HostStatus
	if req.Enable {
		newStatus = models.HostStatusDraining
	} else {
		newStatus = models.HostStatusReady
	}

	h.DB.Model(&host).Update("status", newStatus)

	h.Hub.Broadcast(websocket.Event{
		Type:     "host.maintenance",
		Resource: "host",
		ID:       id.String(),
		Data:     map[string]interface{}{"status": newStatus, "maintenance": req.Enable},
	})

	c.JSON(http.StatusOK, gin.H{"status": newStatus})
}

// ── Clusters ──

func (h *HostHandler) ListClusters(c *gin.Context) {
	var clusters []models.Cluster
	h.DB.Preload("Hosts").Find(&clusters)
	c.JSON(http.StatusOK, clusters)
}

func (h *HostHandler) GetCluster(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid cluster ID"})
		return
	}
	var cluster models.Cluster
	if err := h.DB.Preload("Hosts").First(&cluster, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Cluster not found"})
		return
	}
	c.JSON(http.StatusOK, cluster)
}

func (h *HostHandler) CreateCluster(c *gin.Context) {
	var cluster models.Cluster
	if err := c.ShouldBindJSON(&cluster); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.DB.Create(&cluster).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create cluster"})
		return
	}
	c.JSON(http.StatusCreated, cluster)
}
