package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/novasphere/novasphere/internal/config"
	"github.com/novasphere/novasphere/internal/models"
	"github.com/novasphere/novasphere/internal/services/nativehost"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type nativeBroker interface {
	Call(context.Context, nativehost.Request) (json.RawMessage, error)
}
type NativeHandler struct {
	DB      *gorm.DB
	Hosts   map[string]config.NativeHostConfig
	Connect func(config.NativeHostConfig) (nativeBroker, error)
}

func NewNativeHandler(db *gorm.DB, hosts map[string]config.NativeHostConfig) *NativeHandler {
	return &NativeHandler{DB: db, Hosts: hosts, Connect: func(cfg config.NativeHostConfig) (nativeBroker, error) {
		return nativehost.New(nativehost.Config{Address: cfg.Address, User: cfg.User, PrivateKeyFile: cfg.PrivateKeyFile, KnownHostsFile: cfg.KnownHostsFile})
	}}
}

type nativeHostInventory struct {
	Hostname           string   `json:"hostname"`
	Version            string   `json:"version"`
	OS                 string   `json:"os"`
	Kernel             string   `json:"kernel"`
	CPU                int      `json:"cpu"`
	CPUModel           string   `json:"cpuModel"`
	PhysicalInterfaces []string `json:"physicalInterfaces"`
	Memory             struct {
		Total int64 `json:"total"`
	} `json:"memory"`
}

// IsEnrolled reports whether the fixed native broker has an explicit configuration.
func (h *NativeHandler) IsEnrolled(name string) bool {
	_, ok := h.Hosts[name]
	return ok
}

// LiveInventory reads and validates current host facts. It never falls back to
// cached values; callers must treat an error as unavailable telemetry.
func (h *NativeHandler) LiveInventory(ctx context.Context, name string) (nativeHostInventory, error) {
	var inventory nativeHostInventory
	cfg, ok := h.Hosts[name]
	if !ok {
		return inventory, fmt.Errorf("host is not enrolled")
	}
	broker, err := h.Connect(cfg)
	if err != nil {
		return inventory, fmt.Errorf("host enrollment is unavailable")
	}
	result, err := broker.Call(ctx, nativehost.Request{Operation: "inventory", Arguments: map[string]any{}})
	if err != nil {
		return inventory, fmt.Errorf("live host inventory unavailable")
	}
	if err := json.Unmarshal(result, &inventory); err != nil {
		return inventory, fmt.Errorf("invalid host inventory")
	}
	if inventory.Hostname == "" || inventory.CPU < 1 || inventory.Memory.Total < 1 || inventory.Kernel == "" || (inventory.OS == "" && inventory.Version == "") {
		return nativeHostInventory{}, fmt.Errorf("incomplete host inventory")
	}
	return inventory, nil
}

// StorageRegistration reads the sanitized Ceph registration state through an
// enrolled broker. The hypervisor broker allowlists this query; it does not
// provide an arbitrary remote-file interface. Callers must still validate the
// manifest's ownership-independent schema, freshness, and host evidence before
// publishing any backend as usable.
func (h *NativeHandler) StorageRegistration(ctx context.Context, name string) (json.RawMessage, error) {
	cfg, ok := h.Hosts[name]
	if !ok {
		return nil, fmt.Errorf("host is not enrolled")
	}
	broker, err := h.Connect(cfg)
	if err != nil {
		return nil, fmt.Errorf("host enrollment is unavailable")
	}
	result, err := broker.Call(ctx, nativehost.Request{Operation: "ceph.telemetry", Arguments: map[string]any{"query": "storage registration"}})
	if err != nil {
		return nil, fmt.Errorf("Ceph storage registration unavailable")
	}
	if !json.Valid(result) {
		return nil, fmt.Errorf("invalid Ceph storage registration")
	}
	return result, nil
}

func (h *NativeHandler) List(c *gin.Context) {
	names := make([]string, 0, len(h.Hosts))
	for name := range h.Hosts {
		names = append(names, name)
	}
	sort.Strings(names)
	c.JSON(http.StatusOK, gin.H{"hosts": names})
}
func (h *NativeHandler) broker(c *gin.Context) nativeBroker {
	cfg, ok := h.Hosts[c.Param("host")]
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "Host is not enrolled", "outcome": "not_started"})
		return nil
	}
	broker, err := h.Connect(cfg)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Host enrollment is unavailable", "outcome": "not_started"})
		return nil
	}
	return broker
}
func (h *NativeHandler) Inventory(c *gin.Context) {
	broker := h.broker(c)
	if broker == nil {
		return
	}
	result, err := broker.Call(c.Request.Context(), nativehost.Request{Operation: "inventory", Arguments: map[string]any{}})
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Live host inventory unavailable"})
		return
	}
	c.Data(http.StatusOK, "application/json", result)
}

// Operations are explicit product actions, not an arbitrary remote shell.
var nativeOperations = map[string]bool{
	"vm-create": true, "vm-action": true, "vm-edit": true, "vm-attach-disk": true,
	"pool-create": true, "volume-create": true, "network-create": true, "network-action": true,
}

func (h *NativeHandler) Operate(c *gin.Context) {
	var request nativehost.Request
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1024*1024)
	if err := c.ShouldBindJSON(&request); err != nil || !nativeOperations[request.Operation] || request.Arguments == nil {
		c.JSON(400, gin.H{"error": "Invalid native operation", "outcome": "not_started"})
		return
	}
	id, err := uuid.Parse(c.GetHeader("Idempotency-Key"))
	if err != nil {
		c.JSON(400, gin.H{"error": "Idempotency-Key must be a UUID", "outcome": "not_started"})
		return
	}
	value, ok := c.Get("user_id")
	actor, valid := value.(uuid.UUID)
	if !ok || !valid || actor == uuid.Nil {
		c.JSON(401, gin.H{"error": "Authenticated actor required"})
		return
	}
	broker := h.broker(c)
	if broker == nil {
		return
	}
	raw, _ := json.Marshal(request)
	digest := sha256.Sum256(raw)
	task := models.NativeTask{ID: id, ActorID: actor, Host: c.Param("host"), Operation: request.Operation, RequestHash: hex.EncodeToString(digest[:]), Status: "Running"}
	inserted := h.DB.WithContext(c.Request.Context()).Clauses(clause.OnConflict{DoNothing: true}).Create(&task)
	if inserted.Error != nil {
		c.JSON(503, gin.H{"error": "Operation could not be recorded; host was not changed", "outcome": "not_started"})
		return
	}
	if inserted.RowsAffected == 0 {
		var existing models.NativeTask
		if err := h.DB.WithContext(c.Request.Context()).First(&existing, "id = ?", id).Error; err != nil {
			c.JSON(503, gin.H{"error": "Task readback unavailable"})
			return
		}
		if existing.ActorID != actor || existing.Host != task.Host || existing.RequestHash != task.RequestHash {
			c.JSON(409, gin.H{"error": "Idempotency key already belongs to another operation", "outcome": "not_started"})
			return
		}
		c.JSON(200, gin.H{"task": existing, "replayed": true})
		return
	}
	result, callErr := broker.Call(c.Request.Context(), request)
	task.Status = "Succeeded"
	// A disconnected caller cannot establish whether a remote mutation completed.
	// Retain Unknown for explicit reconciliation, never infer safe automatic retry.
	if callErr != nil {
		task.Status = "Unknown"
		var rejected *nativehost.BrokerError
		if errors.As(callErr, &rejected) {
			task.Status = "Failed"
		}
	}
	saveCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := h.DB.WithContext(saveCtx).Model(&task).Update("status", task.Status).Error; err != nil {
		c.JSON(503, gin.H{"task_id": id, "error": "Host operation attempted; final task state could not be recorded. Reconcile before retrying."})
		return
	}
	if callErr != nil {
		c.JSON(502, gin.H{"task": task, "error": "Host outcome requires readback before retrying"})
		return
	}
	c.JSON(200, gin.H{"task": task, "result": result})
}
func (h *NativeHandler) Task(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid task ID"})
		return
	}
	var task models.NativeTask
	err = h.DB.WithContext(c.Request.Context()).First(&task, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(404, gin.H{"error": "Task not found"})
		return
	}
	if err != nil {
		c.JSON(503, gin.H{"error": "Task store unavailable"})
		return
	}
	c.JSON(200, task)
}

func (h *NativeHandler) Metrics(c *gin.Context) {
	scope, metric, window := c.DefaultQuery("scope", "host"), c.DefaultQuery("metric", "cpu"), c.DefaultQuery("range", "1h")
	catalogs := map[string]map[string]bool{
		"host":       {"cpu": true, "memory": true, "network_rx": true, "network_tx": true, "disk_read": true, "disk_write": true, "disk_read_iops": true, "disk_write_iops": true, "disk_free": true, "disk_read_latency": true, "disk_write_latency": true},
		"vm":         {"cpu": true, "memory": true, "network_rx": true, "network_tx": true, "disk_read": true, "disk_write": true, "disk_read_iops": true, "disk_write_iops": true, "disk_read_latency": true, "disk_write_latency": true},
		"containers": {"cpu": true, "memory": true, "network_rx": true, "network_tx": true, "restarts": true, "nodes_ready": true},
		"storage":    {"health": true, "used": true, "capacity": true, "osd_up": true, "read": true, "write": true, "latency": true},
	}
	if !catalogs[scope][metric] || (window != "1h" && window != "6h" && window != "24h" && window != "7d") {
		c.JSON(400, gin.H{"error": "Unsupported monitoring query"})
		return
	}
	if scope == "vm" && c.Query("name") == "" {
		c.JSON(400, gin.H{"error": "VM name is required"})
		return
	}
	broker := h.broker(c)
	if broker == nil {
		return
	}
	result, err := broker.Call(c.Request.Context(), nativehost.Request{Operation: "metrics", Arguments: map[string]any{"scope": scope, "metric": metric, "range": window, "name": c.Query("name"), "namespace": c.Query("namespace"), "pod": c.Query("pod")}})
	if err != nil {
		c.JSON(502, gin.H{"error": "Host monitoring unavailable"})
		return
	}
	c.Data(200, "application/json", result)
}

func (h *NativeHandler) Files(c *gin.Context) {
	broker := h.broker(c)
	if broker == nil {
		return
	}
	result, err := broker.Call(c.Request.Context(), nativehost.Request{Operation: "files", Arguments: map[string]any{"path": c.Query("path")}})
	if err != nil {
		c.JSON(502, gin.H{"error": "Guest datastore listing unavailable"})
		return
	}
	c.Data(200, "application/json", result)
}
