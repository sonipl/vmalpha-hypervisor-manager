package handlers

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/novasphere/novasphere/internal/models"
	"github.com/novasphere/novasphere/internal/websocket"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type HostHandler struct {
	DB     *gorm.DB
	Hub    *websocket.Hub
	Log    *logrus.Logger
	Native *NativeHandler
}

// recoveredNativeStatus restores a host that was marked Offline solely because
// its last live broker request failed.  A current, validated inventory response
// is stronger evidence than that transient failure.  Operator-selected states
// remain untouched.
func recoveredNativeStatus(status models.HostStatus) models.HostStatus {
	if status == models.HostStatusOffline || status == models.HostStatusError {
		return models.HostStatusReady
	}
	return status
}

func NewHostHandler(db *gorm.DB, hub *websocket.Hub, log *logrus.Logger) *HostHandler {
	return &HostHandler{DB: db, Hub: hub, Log: log}
}

func (h *HostHandler) nativeName(host models.Host) string {
	if h.Native == nil {
		return ""
	}
	for name, cfg := range h.Native.Hosts {
		if host.Name == name || strings.SplitN(host.Name, ".", 2)[0] == name {
			return name
		}
		if value, ok := host.Labels["hostname"].(string); ok && strings.SplitN(value, ".", 2)[0] == name {
			return name
		}
		if address := strings.SplitN(cfg.Address, ":", 2)[0]; address == host.IPAddress {
			return name
		}
	}
	return ""
}

// refreshNativeHosts updates only facts from a successful current broker read.
// Failed reads clear dynamic facts and mark Ready hosts Offline, while retaining
// cluster membership and any Maintenance/Draining operator state.
func (h *HostHandler) refreshNativeHosts(c *gin.Context) {
	if h.Native == nil {
		return
	}
	var hosts []models.Host
	if err := h.DB.Find(&hosts).Error; err != nil {
		h.Log.WithError(err).Warn("native host refresh query failed")
		return
	}
	for _, host := range hosts {
		name := h.nativeName(host)
		if name == "" {
			continue
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
		inventory, err := h.Native.LiveInventory(ctx, name)
		cancel()
		updates := map[string]any{}
		if err != nil {
			updates = map[string]any{"cpu_model": "", "cpu_cores": 0, "memory_mb": 0, "nic_count": 0, "kernel_ver": "", "os_version": "", "osd_count": 0}
			if host.Status == models.HostStatusReady {
				updates["status"] = models.HostStatusOffline
			}
			h.Log.WithError(err).WithField("host", host.Name).Warn("native host inventory unavailable")
		} else {
			osVersion := inventory.OS
			if osVersion == "" {
				osVersion = inventory.Version
			}
			updates = map[string]any{"cpu_model": inventory.CPUModel, "cpu_cores": inventory.CPU, "memory_mb": int(inventory.Memory.Total / (1024 * 1024)), "nic_count": len(inventory.PhysicalInterfaces), "kernel_ver": inventory.Kernel, "os_version": osVersion, "status": recoveredNativeStatus(host.Status)}
		}
		if err := h.DB.Model(&models.Host{}).Where("id = ?", host.ID).Updates(updates).Error; err != nil {
			h.Log.WithError(err).WithField("host", host.Name).Warn("native host fact update failed")
		}
	}
}

func (h *HostHandler) ListHosts(c *gin.Context) {
	sort, err := validatedSort(c.Query("sort"), c.Query("order"), hostSortFields)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	syncHostsFromKubernetes(h.DB, h.Log)
	h.refreshNativeHosts(c)

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
	if name := h.nativeName(host); name != "" {
		if inventory, err := h.Native.LiveInventory(c.Request.Context(), name); err == nil {
			osVersion := inventory.OS
			if osVersion == "" {
				osVersion = inventory.Version
			}
			h.DB.Model(&host).Updates(map[string]any{"cpu_model": inventory.CPUModel, "cpu_cores": inventory.CPU, "memory_mb": int(inventory.Memory.Total / (1024 * 1024)), "nic_count": len(inventory.PhysicalInterfaces), "kernel_ver": inventory.Kernel, "os_version": osVersion, "status": recoveredNativeStatus(host.Status)})
			h.DB.First(&host, "id = ?", id)
		} else {
			updates := map[string]any{"cpu_model": "", "cpu_cores": 0, "memory_mb": 0, "nic_count": 0, "kernel_ver": "", "os_version": "", "osd_count": 0}
			if host.Status == models.HostStatusReady {
				updates["status"] = models.HostStatusOffline
			}
			h.DB.Model(&host).Updates(updates)
			h.DB.First(&host, "id = ?", id)
			h.Log.WithError(err).WithField("host", host.Name).Warn("native host inventory unavailable")
		}
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
