package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/novasphere/novasphere/internal/models"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type StorageHandler struct {
	DB  *gorm.DB
	Log *logrus.Logger
}

func NewStorageHandler(db *gorm.DB, log *logrus.Logger) *StorageHandler {
	return &StorageHandler{DB: db, Log: log}
}

// Storage Classes
func (h *StorageHandler) ListStorageClasses(c *gin.Context) {
	var classes []models.StorageClass
	h.DB.Find(&classes)
	c.JSON(http.StatusOK, classes)
}

func (h *StorageHandler) CreateStorageClass(c *gin.Context) {
	var sc models.StorageClass
	if err := c.ShouldBindJSON(&sc); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.DB.Create(&sc).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create storage class"})
		return
	}
	c.JSON(http.StatusCreated, sc)
}

func (h *StorageHandler) UpdateStorageClass(c *gin.Context) {
	id, _ := uuid.Parse(c.Param("id"))
	var sc models.StorageClass
	if err := h.DB.First(&sc, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Storage class not found"})
		return
	}
	if err := c.ShouldBindJSON(&sc); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	h.DB.Save(&sc)
	c.JSON(http.StatusOK, sc)
}

// Volumes
func (h *StorageHandler) ListVolumes(c *gin.Context) {
	var volumes []models.Volume
	query := h.DB.Model(&models.Volume{})

	if ns := c.Query("namespace"); ns != "" {
		query = query.Where("namespace = ?", ns)
	}
	if sc := c.Query("storage_class"); sc != "" {
		query = query.Where("storage_class = ?", sc)
	}
	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}

	var total int64
	query.Count(&total)

	page, perPage := parsePagination(c)
	query.Offset((page - 1) * perPage).Limit(perPage).Find(&volumes)

	c.JSON(http.StatusOK, PaginatedResponse{Data: volumes, Total: total, Page: page, PerPage: perPage})
}

func (h *StorageHandler) CreateVolume(c *gin.Context) {
	var vol models.Volume
	if err := c.ShouldBindJSON(&vol); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	vol.Status = "Available"
	if err := h.DB.Create(&vol).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create volume"})
		return
	}
	c.JSON(http.StatusCreated, vol)
}

func (h *StorageHandler) ExpandVolume(c *gin.Context) {
	id, _ := uuid.Parse(c.Param("id"))
	var vol models.Volume
	if err := h.DB.First(&vol, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Volume not found"})
		return
	}

	var req struct {
		NewSizeGB int `json:"new_size_gb" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.NewSizeGB <= vol.SizeGB {
		c.JSON(http.StatusBadRequest, gin.H{"error": "New size must be larger than current size"})
		return
	}

	h.DB.Model(&vol).Update("size_gb", req.NewSizeGB)
	c.JSON(http.StatusOK, gin.H{"message": "Volume expanded", "new_size_gb": req.NewSizeGB})
}

func (h *StorageHandler) DeleteVolume(c *gin.Context) {
	id, _ := uuid.Parse(c.Param("id"))
	var vol models.Volume
	if err := h.DB.First(&vol, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Volume not found"})
		return
	}
	if vol.VMID != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Volume is attached to a VM, detach first"})
		return
	}
	h.DB.Delete(&vol)
	c.Status(http.StatusNoContent)
}

// Snapshots
func (h *StorageHandler) ListSnapshots(c *gin.Context) {
	var snapshots []models.VMSnapshot
	query := h.DB.Model(&models.VMSnapshot{})
	if vmID := c.Query("vm_id"); vmID != "" {
		query = query.Where("vm_id = ?", vmID)
	}
	query.Order("created_at DESC").Find(&snapshots)
	c.JSON(http.StatusOK, snapshots)
}

func (h *StorageHandler) RestoreSnapshot(c *gin.Context) {
	id, _ := uuid.Parse(c.Param("id"))
	var snap models.VMSnapshot
	if err := h.DB.First(&snap, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Snapshot not found"})
		return
	}
	// In production: trigger Ceph RBD snapshot restore via KubeVirt
	c.JSON(http.StatusOK, gin.H{"message": "Snapshot restore initiated", "snapshot": snap.Name, "vm_id": snap.VMID})
}

func (h *StorageHandler) DeleteSnapshot(c *gin.Context) {
	id, _ := uuid.Parse(c.Param("id"))
	if err := h.DB.Delete(&models.VMSnapshot{}, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete snapshot"})
		return
	}
	c.Status(http.StatusNoContent)
}

// Backup Jobs
func (h *StorageHandler) ListBackupJobs(c *gin.Context) {
	var jobs []models.BackupJob
	h.DB.Find(&jobs)
	c.JSON(http.StatusOK, jobs)
}

func (h *StorageHandler) CreateBackupJob(c *gin.Context) {
	var job models.BackupJob
	if err := c.ShouldBindJSON(&job); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.DB.Create(&job).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create backup job"})
		return
	}
	c.JSON(http.StatusCreated, job)
}
