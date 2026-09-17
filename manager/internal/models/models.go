package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// BaseModel provides common fields for all models
type BaseModel struct {
	ID        uuid.UUID      `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

// ──────────────────────────────────────────
// Virtual Machine
// ──────────────────────────────────────────

type VMStatus string

const (
	VMStatusRunning      VMStatus = "Running"
	VMStatusStopped      VMStatus = "Stopped"
	VMStatusPaused       VMStatus = "Paused"
	VMStatusMigrating    VMStatus = "Migrating"
	VMStatusProvisioning VMStatus = "Provisioning"
	VMStatusError        VMStatus = "Error"
)

type VirtualMachine struct {
	BaseModel
	Name          string     `json:"name" gorm:"uniqueIndex:idx_vm_ns_name"`
	Namespace     string     `json:"namespace" gorm:"uniqueIndex:idx_vm_ns_name;index"`
	Status        VMStatus   `json:"status" gorm:"index"`
	Description   string     `json:"description"`
	OS            string     `json:"os"`
	OSVersion     string     `json:"os_version"`
	VCPUs         int        `json:"vcpus"`
	MemoryMB      int        `json:"memory_mb"`
	CPUSockets    int        `json:"cpu_sockets"`
	CPUCores      int        `json:"cpu_cores"`
	CPUThreads    int        `json:"cpu_threads"`
	CPUModel      string     `json:"cpu_model"`
	HostNode      string     `json:"host_node"`
	IPAddress     string     `json:"ip_address"`
	BootOrder     string     `json:"boot_order" gorm:"default:'disk,network'"`
	SecureBoot    bool       `json:"secure_boot"`
	VTPM          bool       `json:"vtpm"`
	GPUDevice     string     `json:"gpu_device"`
	EvictionStrat string     `json:"eviction_strategy" gorm:"default:'LiveMigrate'"`
	TemplateID    *uuid.UUID `json:"template_id"`
	OwnerID       uuid.UUID  `json:"owner_id" gorm:"index"`
	TenantID      uuid.UUID  `json:"tenant_id" gorm:"index"`
	Labels        JSONMap    `json:"labels" gorm:"type:jsonb;default:'{}'"`
	Annotations   JSONMap    `json:"annotations" gorm:"type:jsonb;default:'{}'"`
	CloudInit     string     `json:"cloud_init" gorm:"type:text"`

	// Relations
	Disks  []VMDisk `json:"disks" gorm:"foreignKey:VMID"`
	NICs   []VMNIC  `json:"nics" gorm:"foreignKey:VMID"`
	Owner  User     `json:"-" gorm:"foreignKey:OwnerID"`
	Tenant Tenant   `json:"-" gorm:"foreignKey:TenantID"`
}

type VMDisk struct {
	BaseModel
	VMID         uuid.UUID `json:"vm_id" gorm:"index"`
	Name         string    `json:"name"`
	SizeGB       int       `json:"size_gb"`
	StorageClass string    `json:"storage_class"`
	CephPool     string    `json:"ceph_pool"`
	Bus          string    `json:"bus" gorm:"default:'virtio'"`
	CacheMode    string    `json:"cache_mode" gorm:"default:'none'"`
	Bootable     bool      `json:"bootable"`
	HotPlugged   bool      `json:"hot_plugged"`
	PVUID        string    `json:"pv_uid"`
}

type VMNIC struct {
	BaseModel
	VMID       uuid.UUID `json:"vm_id" gorm:"index"`
	Name       string    `json:"name"`
	NetworkID  uuid.UUID `json:"network_id"`
	MACAddress string    `json:"mac_address"`
	Model      string    `json:"model" gorm:"default:'virtio'"`
	IPAddress  string    `json:"ip_address"`
	Type       string    `json:"type" gorm:"default:'bridge'"` // bridge, masquerade, sriov
	HotPlugged bool      `json:"hot_plugged"`
}

type VMSnapshot struct {
	BaseModel
	VMID        uuid.UUID `json:"vm_id" gorm:"index"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	SizeMB      int64     `json:"size_mb"`
	Status      string    `json:"status"` // Creating, Ready, Error
	CreatedBy   uuid.UUID `json:"created_by"`
}

// ──────────────────────────────────────────
// Templates
// ──────────────────────────────────────────

type VMTemplate struct {
	BaseModel
	Name         string     `json:"name" gorm:"uniqueIndex"`
	Description  string     `json:"description"`
	Category     string     `json:"category"` // linux-server, windows-server, etc.
	OS           string     `json:"os"`
	OSVersion    string     `json:"os_version"`
	VCPUs        int        `json:"vcpus"`
	MemoryMB     int        `json:"memory_mb"`
	DiskGB       int        `json:"disk_gb"`
	StorageClass string     `json:"storage_class"`
	CloudInit    string     `json:"cloud_init" gorm:"type:text"`
	ImageURL     string     `json:"image_url"`
	IsPublic     bool       `json:"is_public" gorm:"index"`
	TenantID     *uuid.UUID `json:"tenant_id" gorm:"index"`
	Labels       JSONMap    `json:"labels" gorm:"type:jsonb;default:'{}'"`
}

// ──────────────────────────────────────────
// Compute / Hosts
// ──────────────────────────────────────────

type HostStatus string

const (
	HostStatusReady       HostStatus = "Ready"
	HostStatusMaintenance HostStatus = "Maintenance"
	HostStatusDraining    HostStatus = "Draining"
	HostStatusError       HostStatus = "Error"
	HostStatusOffline     HostStatus = "Offline"
)

type Host struct {
	BaseModel
	Name       string     `json:"name" gorm:"uniqueIndex"`
	Status     HostStatus `json:"status" gorm:"index"`
	IPAddress  string     `json:"ip_address"`
	ClusterID  uuid.UUID  `json:"cluster_id" gorm:"index"`
	CPUModel   string     `json:"cpu_model"`
	CPUCores   int        `json:"cpu_cores"`
	MemoryMB   int        `json:"memory_mb"`
	NICCount   int        `json:"nic_count"`
	GPUDevices JSONMap    `json:"gpu_devices" gorm:"type:jsonb;default:'[]'"`
	OSDCount   int        `json:"osd_count"`
	KernelVer  string     `json:"kernel_version"`
	OSVersion  string     `json:"os_version"`
	Labels     JSONMap    `json:"labels" gorm:"type:jsonb;default:'{}'"`
	BMCAddress string     `json:"bmc_address"`
	NUMANodes  int        `json:"numa_nodes"`
}

type Cluster struct {
	BaseModel
	Name                string  `json:"name" gorm:"uniqueIndex"`
	Description         string  `json:"description"`
	CPUOvercommit       float64 `json:"cpu_overcommit" gorm:"default:4.0"`
	MemoryOvercommit    float64 `json:"memory_overcommit" gorm:"default:1.5"`
	DefaultStorageClass string  `json:"default_storage_class"`
	Hosts               []Host  `json:"hosts" gorm:"foreignKey:ClusterID"`
}

// ──────────────────────────────────────────
// Storage
// ──────────────────────────────────────────

type StorageClass struct {
	BaseModel
	Name            string `json:"name" gorm:"uniqueIndex"`
	Description     string `json:"description"`
	Provisioner     string `json:"provisioner"` // rbd.csi.ceph.com
	CephPool        string `json:"ceph_pool"`
	ReplicaCount    int    `json:"replica_count" gorm:"default:3"`
	IsDefault       bool   `json:"is_default"`
	MaxIOPS         int    `json:"max_iops"`
	MaxThroughputMB int    `json:"max_throughput_mb"`
	Encryption      bool   `json:"encryption"`
	Compression     bool   `json:"compression"`
	ErasureCoded    bool   `json:"erasure_coded"`
}

type Volume struct {
	BaseModel
	Name         string     `json:"name" gorm:"uniqueIndex"`
	Namespace    string     `json:"namespace" gorm:"index"`
	SizeGB       int        `json:"size_gb"`
	UsedGB       float64    `json:"used_gb"`
	StorageClass string     `json:"storage_class"`
	CephPool     string     `json:"ceph_pool"`
	Status       string     `json:"status"` // Bound, Available, Released, Failed
	AccessMode   string     `json:"access_mode"`
	VMID         *uuid.UUID `json:"vm_id" gorm:"index"`
}

// ──────────────────────────────────────────
// Networking
// ──────────────────────────────────────────

type Network struct {
	BaseModel
	Name           string    `json:"name" gorm:"uniqueIndex:idx_net_ns_name"`
	Namespace      string    `json:"namespace" gorm:"uniqueIndex:idx_net_ns_name;index"`
	Type           string    `json:"type"` // bridge, sriov, macvtap
	VLANID         int       `json:"vlan_id"`
	Subnet         string    `json:"subnet"`
	Gateway        string    `json:"gateway"`
	DHCPEnabled    bool      `json:"dhcp_enabled"`
	DHCPRangeStart string    `json:"dhcp_range_start"`
	DHCPRangeEnd   string    `json:"dhcp_range_end"`
	MTU            int       `json:"mtu" gorm:"default:1500"`
	TenantID       uuid.UUID `json:"tenant_id" gorm:"index"`
}

type FirewallRule struct {
	BaseModel
	Name      string `json:"name"`
	Namespace string `json:"namespace" gorm:"index"`
	Direction string `json:"direction"` // Ingress, Egress
	Protocol  string `json:"protocol"`  // TCP, UDP, ICMP
	PortRange string `json:"port_range"`
	Source    string `json:"source"`
	Dest      string `json:"dest"`
	Action    string `json:"action"` // Allow, Deny
	Priority  int    `json:"priority"`
}

// ──────────────────────────────────────────
// Auth / RBAC
// ──────────────────────────────────────────

type User struct {
	BaseModel
	Username   string     `json:"username" gorm:"uniqueIndex"`
	Email      string     `json:"email" gorm:"uniqueIndex"`
	FullName   string     `json:"full_name"`
	Password   string     `json:"-"` // bcrypt hashed
	RoleID     uuid.UUID  `json:"role_id" gorm:"index"`
	TenantID   uuid.UUID  `json:"tenant_id" gorm:"index"`
	IsActive   bool       `json:"is_active" gorm:"default:true"`
	MFAEnabled bool       `json:"mfa_enabled"`
	MFASecret  string     `json:"-"`
	LastLogin  *time.Time `json:"last_login"`
	ExternalID string     `json:"external_id"`                   // LDAP/OIDC subject
	Source     string     `json:"source" gorm:"default:'local'"` // local, ldap, oidc

	Role   Role   `json:"role" gorm:"foreignKey:RoleID"`
	Tenant Tenant `json:"tenant" gorm:"foreignKey:TenantID"`
}

type Role struct {
	BaseModel
	Name        string  `json:"name" gorm:"uniqueIndex"`
	Description string  `json:"description"`
	Scope       string  `json:"scope"` // Global, Cluster, Namespace
	IsBuiltIn   bool    `json:"is_built_in"`
	Permissions JSONMap `json:"permissions" gorm:"type:jsonb;default:'[]'"`
}

type Permission struct {
	Resource string `json:"resource"` // virtualmachines, hosts, storage, etc.
	Verb     string `json:"verb"`     // create, read, update, delete, list, console, migrate
	Scope    string `json:"scope"`    // *, namespace:<name>, cluster:<name>
}

type Tenant struct {
	BaseModel
	Name           string     `json:"name" gorm:"uniqueIndex"`
	Description    string     `json:"description"`
	MaxVCPUs       int        `json:"max_vcpus"`
	MaxMemoryMB    int        `json:"max_memory_mb"`
	MaxStorageGB   int        `json:"max_storage_gb"`
	MaxVMs         int        `json:"max_vms"`
	MaxSnapshots   int        `json:"max_snapshots"`
	ParentID       *uuid.UUID `json:"parent_id"` // Hierarchical tenancy
	CustomBranding JSONMap    `json:"custom_branding" gorm:"type:jsonb;default:'{}'"`
}

type AuditLog struct {
	ID         uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	Timestamp  time.Time `json:"timestamp" gorm:"index"`
	UserID     uuid.UUID `json:"user_id" gorm:"index"`
	Username   string    `json:"username"`
	Action     string    `json:"action" gorm:"index"`   // create, delete, start, stop, login, etc.
	Resource   string    `json:"resource" gorm:"index"` // vm, host, network, etc.
	ResourceID string    `json:"resource_id"`
	Details    JSONMap   `json:"details" gorm:"type:jsonb;default:'{}'"`
	IPAddress  string    `json:"ip_address"`
	Source     string    `json:"source"` // ui, api, ai-assistant, automation
	Success    bool      `json:"success"`
}

// ──────────────────────────────────────────
// AI / NovaMind
// ──────────────────────────────────────────

type AIRecommendation struct {
	BaseModel
	Type        string     `json:"type" gorm:"index"` // right-size, placement, migration, anomaly, capacity
	Severity    string     `json:"severity"`          // info, warning, critical
	TargetType  string     `json:"target_type"`       // vm, host, cluster, storage
	TargetID    uuid.UUID  `json:"target_id" gorm:"index"`
	TargetName  string     `json:"target_name"`
	Title       string     `json:"title"`
	Description string     `json:"description" gorm:"type:text"`
	Confidence  float64    `json:"confidence"`
	Status      string     `json:"status" gorm:"default:'pending'"` // pending, applied, dismissed
	ActionJSON  JSONMap    `json:"action_json" gorm:"type:jsonb;default:'{}'"`
	AppliedBy   *uuid.UUID `json:"applied_by"`
	AppliedAt   *time.Time `json:"applied_at"`
}

type AutomationPolicy struct {
	BaseModel
	Name        string     `json:"name" gorm:"uniqueIndex"`
	Type        string     `json:"type"` // load-balance, right-size, storage-tier, preemptive-migrate, patch
	Enabled     bool       `json:"enabled" gorm:"default:false"`
	AutoApply   bool       `json:"auto_apply" gorm:"default:false"` // true = auto, false = manual approval
	Trigger     JSONMap    `json:"trigger" gorm:"type:jsonb"`       // conditions
	Action      JSONMap    `json:"action" gorm:"type:jsonb"`        // what to do
	CooldownMin int        `json:"cooldown_min" gorm:"default:30"`
	LastRunAt   *time.Time `json:"last_run_at"`
}

// ──────────────────────────────────────────
// Migration
// ──────────────────────────────────────────

type MigrationPlan struct {
	BaseModel
	Name         string            `json:"name"`
	SourceType   string            `json:"source_type"` // vmware, hyperv, olvm, ovirt
	SourceConfig JSONMap           `json:"source_config" gorm:"type:jsonb"`
	Status       string            `json:"status"` // Planning, Ready, InProgress, Completed, Failed
	VMs          []MigrationPlanVM `json:"vms" gorm:"foreignKey:PlanID"`
	CreatedBy    uuid.UUID         `json:"created_by"`
}

type MigrationPlanVM struct {
	BaseModel
	PlanID         uuid.UUID `json:"plan_id" gorm:"index"`
	SourceVMName   string    `json:"source_vm_name"`
	TargetVMName   string    `json:"target_vm_name"`
	Status         string    `json:"status"`          // Pending, Migrating, Completed, Failed
	ReadinessScore float64   `json:"readiness_score"` // 0-1 AI assessment
	Blockers       JSONMap   `json:"blockers" gorm:"type:jsonb;default:'[]'"`
	NetworkMapping JSONMap   `json:"network_mapping" gorm:"type:jsonb;default:'{}'"`
	StorageMapping JSONMap   `json:"storage_mapping" gorm:"type:jsonb;default:'{}'"`
}

// ──────────────────────────────────────────
// Monitoring / Alerts
// ──────────────────────────────────────────

type AlertRule struct {
	BaseModel
	Name        string  `json:"name" gorm:"uniqueIndex"`
	Severity    string  `json:"severity"`                    // critical, warning, info
	Expression  string  `json:"expression" gorm:"type:text"` // PromQL
	Duration    string  `json:"duration"`                    // e.g. "5m"
	Labels      JSONMap `json:"labels" gorm:"type:jsonb;default:'{}'"`
	Annotations JSONMap `json:"annotations" gorm:"type:jsonb;default:'{}'"`
	Enabled     bool    `json:"enabled" gorm:"default:true"`
	Channels    JSONMap `json:"channels" gorm:"type:jsonb;default:'[]'"` // notification channels
}

// Ceph time-series samples for Storage UI (vSAN-like health/perf)
type CephMetricSample struct {
	ID             uint      `json:"id" gorm:"primaryKey"`
	CreatedAt      time.Time `json:"created_at" gorm:"index"`
	Health         string    `json:"health"`
	TotalBytes     int64     `json:"total_bytes"`
	UsedBytes      int64     `json:"used_bytes"`
	AvailBytes     int64     `json:"avail_bytes"`
	NumOSD         int       `json:"num_osd"`
	NumOSDUp       int       `json:"num_osd_up"`
	NumOSDIn       int       `json:"num_osd_in"`
	NumHosts       int       `json:"num_hosts"`
	ReadIOPS       float64   `json:"read_iops"`
	WriteIOPS      float64   `json:"write_iops"`
	ReadLatencyMs  float64   `json:"read_latency_ms"`
	WriteLatencyMs float64   `json:"write_latency_ms"`
	RawJSON        string    `json:"-" gorm:"type:text"`
}

type BackupJob struct {
	BaseModel
	Name       string     `json:"name"`
	VMID       uuid.UUID  `json:"vm_id" gorm:"index"`
	Schedule   string     `json:"schedule"` // cron expression
	Retention  int        `json:"retention_days"`
	Type       string     `json:"type"` // full, incremental
	TargetPool string     `json:"target_pool"`
	LastRunAt  *time.Time `json:"last_run_at"`
	LastStatus string     `json:"last_status"`
	Enabled    bool       `json:"enabled" gorm:"default:true"`
}

// ──────────────────────────────────────────
// JSON helper type for GORM jsonb
// ──────────────────────────────────────────

type JSONMap map[string]interface{}

// NativeTask records intent before a remote operation. An interrupted Running
// task must be reconciled by readback; it is never automatically repeated.
type NativeTask struct {
	ID          uuid.UUID `json:"id" gorm:"type:uuid;primaryKey"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	ActorID     uuid.UUID `json:"actor_id" gorm:"type:uuid;index"`
	Host        string    `json:"host"`
	Operation   string    `json:"operation"`
	RequestHash string    `json:"-"`
	Status      string    `json:"status" gorm:"index"`
}
