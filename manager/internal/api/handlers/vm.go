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

type VMHandler struct {
	DB  *gorm.DB
	Hub *websocket.Hub
	Log *logrus.Logger
}

func NewVMHandler(db *gorm.DB, hub *websocket.Hub, log *logrus.Logger) *VMHandler {
	return &VMHandler{DB: db, Hub: hub, Log: log}
}

// ListVMs godoc
// @Summary List virtual machines
// @Description List all VMs with filtering, sorting, and pagination
// @Tags virtual-machines
// @Security BearerAuth
// @Param namespace query string false "Filter by namespace"
// @Param status query string false "Filter by status"
// @Param search query string false "Search by name"
// @Param page query int false "Page number" default(1)
// @Param per_page query int false "Items per page" default(25)
// @Param sort query string false "Sort field" default(name)
// @Param order query string false "Sort order (asc/desc)" default(asc)
// @Success 200 {object} PaginatedResponse{data=[]models.VirtualMachine}
// @Router /vms [get]
func (h *VMHandler) ListVMs(c *gin.Context) {
	sort, err := validatedSort(c.Query("sort"), c.Query("order"), vmSortFields)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	syncVMsFromKubeVirt(h.DB, h.Log)

	var vms []models.VirtualMachine
	query := h.DB.Preload("Disks").Preload("NICs")

	// Tenant scoping
	tenantID, _ := c.Get("tenant_id")
	role, _ := c.Get("role")
	if role.(string) != "Platform Admin" && role.(string) != "Infrastructure Admin" {
		query = query.Where("tenant_id = ?", tenantID)
	}

	// Filters
	if ns := c.Query("namespace"); ns != "" {
		query = query.Where("namespace = ?", ns)
	}
	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}
	if search := c.Query("search"); search != "" {
		query = query.Where("name ILIKE ?", "%"+search+"%")
	}

	// Count total
	var total int64
	query.Model(&models.VirtualMachine{}).Count(&total)

	// Pagination
	page, perPage := parsePagination(c)
	offset := (page - 1) * perPage

	// Sorting
	query = query.Order(sort)

	if err := query.Offset(offset).Limit(perPage).Find(&vms).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch VMs"})
		return
	}

	c.JSON(http.StatusOK, PaginatedResponse{
		Data:    vms,
		Total:   total,
		Page:    page,
		PerPage: perPage,
	})
}

// GetVM godoc
// @Summary Get a virtual machine
// @Description Get detailed information about a specific VM
// @Tags virtual-machines
// @Security BearerAuth
// @Param id path string true "VM ID"
// @Success 200 {object} models.VirtualMachine
// @Router /vms/{id} [get]
func (h *VMHandler) GetVM(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid VM ID"})
		return
	}

	var vm models.VirtualMachine
	if err := h.DB.Preload("Disks").Preload("NICs").First(&vm, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "VM not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch VM"})
		return
	}

	c.JSON(http.StatusOK, vm)
}

// CreateVM godoc
// @Summary Create a virtual machine
// @Description Create a new VM with the specified configuration
// @Tags virtual-machines
// @Security BearerAuth
// @Param vm body CreateVMRequest true "VM configuration"
// @Success 201 {object} models.VirtualMachine
// @Router /vms [post]
func (h *VMHandler) CreateVM(c *gin.Context) {
	var req CreateVMRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Namespace == "" {
		req.Namespace = "default"
	}

	userID, _ := c.Get("user_id")
	tenantID, _ := c.Get("tenant_id")

	vm := models.VirtualMachine{
		Name:          req.Name,
		Namespace:     req.Namespace,
		Status:        models.VMStatusProvisioning,
		Description:   req.Description,
		OS:            req.OS,
		OSVersion:     req.OSVersion,
		VCPUs:         req.VCPUs,
		MemoryMB:      req.MemoryMB,
		CPUSockets:    req.CPUSockets,
		CPUCores:      req.CPUCores,
		CPUThreads:    req.CPUThreads,
		CPUModel:      req.CPUModel,
		SecureBoot:    req.SecureBoot,
		VTPM:          req.VTPM,
		GPUDevice:     req.GPUDevice,
		EvictionStrat: req.EvictionStrategy,
		TemplateID:    req.TemplateID,
		OwnerID:       userID.(uuid.UUID),
		TenantID:      tenantID.(uuid.UUID),
		Labels:        req.Labels,
		Annotations:   req.Annotations,
		CloudInit:     req.CloudInit,
	}

	// Create disks
	for _, d := range req.Disks {
		vm.Disks = append(vm.Disks, models.VMDisk{
			Name:         d.Name,
			SizeGB:       d.SizeGB,
			StorageClass: d.StorageClass,
			Bus:          d.Bus,
			CacheMode:    d.CacheMode,
			Bootable:     d.Bootable,
		})
	}

	// Create NICs
	for _, n := range req.NICs {
		vm.NICs = append(vm.NICs, models.VMNIC{
			Name:       n.Name,
			NetworkID:  n.NetworkID,
			MACAddress: n.MACAddress,
			Model:      n.Model,
			Type:       n.Type,
		})
	}

	if err := h.DB.Create(&vm).Error; err != nil {
		h.Log.Errorf("Failed to create VM: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create VM"})
		return
	}

	// Broadcast VM creation event
	h.Hub.Broadcast(websocket.Event{
		Type:     "vm.created",
		Resource: "virtualmachine",
		ID:       vm.ID.String(),
		Data:     vm,
	})

	c.JSON(http.StatusCreated, vm)
}

// UpdateVM godoc
// @Summary Update a virtual machine
// @Tags virtual-machines
// @Security BearerAuth
// @Param id path string true "VM ID"
// @Param vm body UpdateVMRequest true "VM updates"
// @Success 200 {object} models.VirtualMachine
// @Router /vms/{id} [put]
func (h *VMHandler) UpdateVM(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid VM ID"})
		return
	}

	var vm models.VirtualMachine
	if err := h.DB.First(&vm, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "VM not found"})
		return
	}

	var req UpdateVMRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updates := map[string]interface{}{}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.VCPUs != nil {
		updates["vcpus"] = *req.VCPUs
	}
	if req.MemoryMB != nil {
		updates["memory_mb"] = *req.MemoryMB
	}
	if req.Labels != nil {
		updates["labels"] = req.Labels
	}
	if req.Annotations != nil {
		updates["annotations"] = req.Annotations
	}

	if err := h.DB.Model(&vm).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update VM"})
		return
	}

	h.DB.Preload("Disks").Preload("NICs").First(&vm, "id = ?", id)

	h.Hub.Broadcast(websocket.Event{
		Type:     "vm.updated",
		Resource: "virtualmachine",
		ID:       vm.ID.String(),
		Data:     vm,
	})

	c.JSON(http.StatusOK, vm)
}

// DeleteVM godoc
// @Summary Delete a virtual machine
// @Tags virtual-machines
// @Security BearerAuth
// @Param id path string true "VM ID"
// @Success 204 "No Content"
// @Router /vms/{id} [delete]
func (h *VMHandler) DeleteVM(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid VM ID"})
		return
	}

	var vm models.VirtualMachine
	if err := h.DB.First(&vm, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "VM not found"})
		return
	}

	// Soft delete (GORM)
	if err := h.DB.Delete(&vm).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete VM"})
		return
	}

	h.Hub.Broadcast(websocket.Event{
		Type:     "vm.deleted",
		Resource: "virtualmachine",
		ID:       id.String(),
	})

	c.Status(http.StatusNoContent)
}

// VMAction godoc
// @Summary Perform an action on a VM
// @Tags virtual-machines
// @Security BearerAuth
// @Param id path string true "VM ID"
// @Param action path string true "Action (start, stop, restart, pause, unpause, migrate, snapshot)"
// @Success 200 {object} gin.H
// @Router /vms/{id}/actions/{action} [post]
func (h *VMHandler) VMAction(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid VM ID"})
		return
	}

	action := c.Param("action")

	var vm models.VirtualMachine
	if err := h.DB.First(&vm, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "VM not found"})
		return
	}

	var newStatus models.VMStatus
	switch action {
	case "start":
		if vm.Status != models.VMStatusStopped {
			c.JSON(http.StatusConflict, gin.H{"error": "VM must be stopped to start"})
			return
		}
		newStatus = models.VMStatusRunning
	case "stop":
		if vm.Status != models.VMStatusRunning && vm.Status != models.VMStatusPaused {
			c.JSON(http.StatusConflict, gin.H{"error": "VM must be running or paused to stop"})
			return
		}
		newStatus = models.VMStatusStopped
	case "restart":
		if vm.Status != models.VMStatusRunning {
			c.JSON(http.StatusConflict, gin.H{"error": "VM must be running to restart"})
			return
		}
		newStatus = models.VMStatusRunning
	case "pause":
		if vm.Status != models.VMStatusRunning {
			c.JSON(http.StatusConflict, gin.H{"error": "VM must be running to pause"})
			return
		}
		newStatus = models.VMStatusPaused
	case "unpause":
		if vm.Status != models.VMStatusPaused {
			c.JSON(http.StatusConflict, gin.H{"error": "VM must be paused to unpause"})
			return
		}
		newStatus = models.VMStatusRunning
	case "migrate":
		var req MigrateVMRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "target_node required"})
			return
		}
		newStatus = models.VMStatusMigrating
		h.DB.Model(&vm).Update("host_node", req.TargetNode)
	case "snapshot":
		var req SnapshotRequest
		c.ShouldBindJSON(&req)
		userID, _ := c.Get("user_id")
		snap := models.VMSnapshot{
			VMID:        id,
			Name:        req.Name,
			Description: req.Description,
			Status:      "Creating",
			CreatedBy:   userID.(uuid.UUID),
		}
		h.DB.Create(&snap)
		c.JSON(http.StatusCreated, snap)
		return
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unknown action: " + action})
		return
	}

	h.DB.Model(&vm).Update("status", newStatus)

	h.Hub.Broadcast(websocket.Event{
		Type:     "vm." + action,
		Resource: "virtualmachine",
		ID:       id.String(),
		Data: map[string]interface{}{
			"status": newStatus,
			"action": action,
		},
	})

	c.JSON(http.StatusOK, gin.H{
		"message": "Action " + action + " executed",
		"status":  newStatus,
	})
}

// ── Request/Response types ──

type CreateVMRequest struct {
	Name             string         `json:"name" binding:"required"`
	Namespace        string         `json:"namespace"`
	Description      string         `json:"description"`
	OS               string         `json:"os" binding:"required"`
	OSVersion        string         `json:"os_version"`
	VCPUs            int            `json:"vcpus" binding:"required,min=1"`
	MemoryMB         int            `json:"memory_mb" binding:"required,min=256"`
	CPUSockets       int            `json:"cpu_sockets"`
	CPUCores         int            `json:"cpu_cores"`
	CPUThreads       int            `json:"cpu_threads"`
	CPUModel         string         `json:"cpu_model"`
	SecureBoot       bool           `json:"secure_boot"`
	VTPM             bool           `json:"vtpm"`
	GPUDevice        string         `json:"gpu_device"`
	EvictionStrategy string         `json:"eviction_strategy"`
	TemplateID       *uuid.UUID     `json:"template_id"`
	Labels           models.JSONMap `json:"labels"`
	Annotations      models.JSONMap `json:"annotations"`
	CloudInit        string         `json:"cloud_init"`
	Disks            []DiskSpec     `json:"disks" binding:"required,min=1"`
	NICs             []NICSpec      `json:"nics"`
}

type DiskSpec struct {
	Name         string `json:"name" binding:"required"`
	SizeGB       int    `json:"size_gb" binding:"required,min=1"`
	StorageClass string `json:"storage_class" binding:"required"`
	Bus          string `json:"bus"`
	CacheMode    string `json:"cache_mode"`
	Bootable     bool   `json:"bootable"`
}

type NICSpec struct {
	Name       string    `json:"name" binding:"required"`
	NetworkID  uuid.UUID `json:"network_id" binding:"required"`
	MACAddress string    `json:"mac_address"`
	Model      string    `json:"model"`
	Type       string    `json:"type"`
}

type UpdateVMRequest struct {
	Description *string        `json:"description"`
	VCPUs       *int           `json:"vcpus"`
	MemoryMB    *int           `json:"memory_mb"`
	Labels      models.JSONMap `json:"labels"`
	Annotations models.JSONMap `json:"annotations"`
}

type MigrateVMRequest struct {
	TargetNode string `json:"target_node" binding:"required"`
}

type SnapshotRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
}

type PaginatedResponse struct {
	Data    interface{} `json:"data"`
	Total   int64       `json:"total"`
	Page    int         `json:"page"`
	PerPage int         `json:"per_page"`
}

func parsePagination(c *gin.Context) (int, int) {
	page := 1
	perPage := 25
	if p := c.Query("page"); p != "" {
		if v := parseInt(p); v > 0 {
			page = v
		}
	}
	if pp := c.Query("per_page"); pp != "" {
		if v := parseInt(pp); v > 0 && v <= 100 {
			perPage = v
		}
	}
	return page, perPage
}

func parseInt(s string) int {
	var v int
	for _, c := range s {
		if c >= '0' && c <= '9' {
			v = v*10 + int(c-'0')
		} else {
			return 0
		}
	}
	return v
}
