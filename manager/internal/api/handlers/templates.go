package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/novasphere/novasphere/internal/models"
	"gorm.io/gorm"
)

type datastoreTemplateRequest struct {
	DatastoreID string `json:"datastore_id" binding:"required"`
	Path        string `json:"path" binding:"required"`
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	Category    string `json:"category"`
	OS          string `json:"os"`
	OSVersion   string `json:"os_version"`
}

type qcowInfo struct {
	Format          string `json:"format"`
	VirtualSize     int64  `json:"virtual-size"`
	BackingFilename string `json:"backing-filename"`
}

// templateDatastorePath permits a qcow2 file immediately below templates/.
// Keeping the placement fixed prevents a template registration request from
// becoming a generic datastore file browser or arbitrary-path probe.
func templateDatastorePath(value string) (string, error) {
	p, err := browserPath(value)
	if err != nil || !strings.HasPrefix(p, "templates/") || path.Dir(p) != "templates" || !strings.HasSuffix(strings.ToLower(p), ".qcow2") {
		return "", fmt.Errorf("template path must be a qcow2 file directly under templates/")
	}
	base := path.Base(p)
	if base == ".qcow2" || strings.HasPrefix(base, ".") {
		return "", fmt.Errorf("invalid template filename")
	}
	return p, nil
}

func parseQCOWInfo(raw []byte) (qcowInfo, error) {
	var info qcowInfo
	if err := json.Unmarshal(raw, &info); err != nil {
		return info, fmt.Errorf("invalid qemu-img metadata")
	}
	if info.Format != "qcow2" || info.VirtualSize < 1 {
		return info, fmt.Errorf("expected a non-empty qcow2 image")
	}
	if strings.TrimSpace(info.BackingFilename) != "" {
		return info, fmt.Errorf("qcow2 backing chains are not accepted as templates")
	}
	return info, nil
}

func inspectQCOW(file *os.File) (qcowInfo, string, error) {
	// qemu-img receives an inherited descriptor, not a user-controlled pathname.
	// The opened descriptor was constrained by os.Root to the registered NFS mount.
	cmd := exec.Command("qemu-img", "info", "--output=json", "/proc/self/fd/3")
	cmd.ExtraFiles = []*os.File{file}
	raw, err := cmd.Output()
	if err != nil {
		return qcowInfo{}, "", fmt.Errorf("qemu-img could not inspect image")
	}
	info, err := parseQCOWInfo(raw)
	if err != nil {
		return qcowInfo{}, "", err
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return qcowInfo{}, "", fmt.Errorf("cannot read image")
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return qcowInfo{}, "", fmt.Errorf("cannot checksum image")
	}
	return info, hex.EncodeToString(hash.Sum(nil)), nil
}

func (h *StorageBackendsHandler) templateRoot(c *gin.Context, datastoreID string) (*os.Root, error) {
	backend, _, ok := backendByID(loadStorageConfig(), datastoreID)
	if !ok || !externalNativeBackend(backend) {
		return nil, fmt.Errorf("registered External NFS datastore required")
	}
	if err := externalMountIdentity(c.Request.Context(), backend); err != nil {
		return nil, fmt.Errorf("datastore is not verified: %w", err)
	}
	target, _, _, err := browserMountConfig(backend)
	if err != nil {
		return nil, err
	}
	return os.OpenRoot(target)
}

// ListTemplates exposes catalog metadata only. It does not browse host paths.
func (h *StorageBackendsHandler) ListTemplates(c *gin.Context) {
	var templates []models.VMTemplate
	query := h.DB.Order("name ASC")
	if tenantID, ok := c.Get("tenant_id"); ok {
		if role, _ := c.Get("role"); role != "Platform Admin" && role != "Infrastructure Admin" {
			query = query.Where("is_public = ? OR tenant_id = ?", true, tenantID)
		}
	}
	if err := query.Find(&templates).Error; err != nil {
		c.JSON(500, gin.H{"error": "cannot list templates"})
		return
	}
	c.JSON(200, gin.H{"data": templates, "items": templates, "total": len(templates)})
}

// RegisterDatastoreTemplate records metadata only after the QCOW2 has been
// safely opened below a registered external datastore's templates directory.
// It never copies, creates, or changes a VM or disk image.
func (h *StorageBackendsHandler) RegisterDatastoreTemplate(c *gin.Context) {
	var req datastoreTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Name) == "" || len(req.Name) > 128 {
		c.JSON(400, gin.H{"error": "datastore_id, template name, and template path are required"})
		return
	}
	p, err := templateDatastorePath(req.Path)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	root, err := h.templateRoot(c, req.DatastoreID)
	if err != nil {
		c.JSON(409, gin.H{"error": err.Error()})
		return
	}
	defer root.Close()
	file, err := root.Open(p)
	if err != nil {
		c.JSON(404, gin.H{"error": "template image not found"})
		return
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil || !stat.Mode().IsRegular() {
		c.JSON(400, gin.H{"error": "regular qcow2 image required"})
		return
	}
	info, checksum, err := inspectQCOW(file)
	if err != nil {
		c.JSON(422, gin.H{"error": err.Error()})
		return
	}
	backend, _, _ := backendByID(loadStorageConfig(), req.DatastoreID)
	imageURL := "datastore://" + req.DatastoreID + "/" + p
	var existing models.VMTemplate
	if err := h.DB.Where("image_url = ?", imageURL).First(&existing).Error; err == nil {
		c.JSON(409, gin.H{"error": "template image is already registered", "template": existing})
		return
	} else if err != nil && err != gorm.ErrRecordNotFound {
		c.JSON(500, gin.H{"error": "cannot read template catalog"})
		return
	}
	template := models.VMTemplate{
		Name: req.Name, Description: req.Description, Category: firstNonEmpty(req.Category, "linux-server"), OS: req.OS, OSVersion: req.OSVersion,
		DiskGB: int((info.VirtualSize + (1 << 30) - 1) / (1 << 30)), StorageClass: backend.StorageClass, ImageURL: imageURL, IsPublic: true,
		Labels: models.JSONMap{"sha256": checksum, "format": "qcow2", "datastore_id": req.DatastoreID, "datastore_path": p, "virtual_size_bytes": info.VirtualSize},
	}
	if tenantID, ok := c.Get("tenant_id"); ok {
		if id, ok := tenantID.(uuid.UUID); ok && id != uuid.Nil {
			template.TenantID = &id
		}
	}
	if err := h.DB.Create(&template).Error; err != nil {
		c.JSON(500, gin.H{"error": "cannot register template metadata"})
		return
	}
	c.JSON(http.StatusCreated, template)
}
