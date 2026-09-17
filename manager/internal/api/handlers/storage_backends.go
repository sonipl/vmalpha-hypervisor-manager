package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

const storageConfigPath = "/var/lib/novasphere/config/storage.json"

var safeNameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]{0,61}[a-z0-9]$`)

type StorageBackend struct {
	ID            string         `json:"id"`
	Type          string         `json:"type"`
	Name          string         `json:"name"`
	StorageClass  string         `json:"storage_class"`
	Status        string         `json:"status"`
	Message       string         `json:"message,omitempty"`
	CapacityBytes int64          `json:"capacity_bytes,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
	Config        map[string]any `json:"config,omitempty"`
}

type storageConfigFile struct {
	Backends []StorageBackend `json:"backends"`
}

type StorageBackendsHandler struct {
	DB  *gorm.DB
	Log *logrus.Logger
}

func NewStorageBackendsHandler(db *gorm.DB, log *logrus.Logger) *StorageBackendsHandler {
	return &StorageBackendsHandler{DB: db, Log: log}
}

func storageConfigFilePath() string {
	// Persist on the mounted config volume (writable). /host is read-only in the API pod.
	for _, p := range []string{storageConfigPath, "/host/var/lib/novasphere/config/storage.json"} {
		if _, err := os.Stat(filepath.Dir(p)); err == nil {
			return p
		}
	}
	return storageConfigPath
}

func loadStorageConfig() storageConfigFile {
	cfg := storageConfigFile{Backends: []StorageBackend{}}
	b, err := os.ReadFile(storageConfigFilePath())
	if err != nil {
		return cfg
	}
	_ = json.Unmarshal(b, &cfg)
	if cfg.Backends == nil {
		cfg.Backends = []StorageBackend{}
	}
	return cfg
}

func saveStorageConfig(cfg storageConfigFile) error {
	path := storageConfigFilePath()
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		return err
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0600)
}

func sanitizeStorageClass(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	name = strings.ReplaceAll(name, "_", "-")
	name = regexp.MustCompile(`[^a-z0-9.-]+`).ReplaceAllString(name, "-")
	name = strings.Trim(name, ".-")
	if name == "" {
		name = "storage"
	}
	return name
}

func backendByID(cfg storageConfigFile, id string) (StorageBackend, int, bool) {
	for i, b := range cfg.Backends {
		if b.ID == id || b.StorageClass == id || b.Name == id {
			return b, i, true
		}
	}
	return StorageBackend{}, -1, false
}

type k8sStorageClass struct {
	Name         string `json:"name"`
	Provisioner  string `json:"provisioner"`
	ReclaimPolicy string `json:"reclaim_policy"`
	VolumeBindingMode string `json:"volume_binding_mode"`
	Default      bool   `json:"is_default"`
}

func listK8sStorageClasses() []k8sStorageClass {
	out, err := kubectlCmd("get", "storageclass", "-o", "json").CombinedOutput()
	if err != nil {
		return nil
	}
	var payload struct {
		Items []struct {
			Metadata struct {
				Name   string            `json:"name"`
				Labels map[string]string `json:"labels"`
				Annotations map[string]string `json:"annotations"`
			} `json:"metadata"`
			Provisioner       string `json:"provisioner"`
			ReclaimPolicy     string `json:"reclaimPolicy"`
			VolumeBindingMode string `json:"volumeBindingMode"`
		} `json:"items"`
	}
	if json.Unmarshal(out, &payload) != nil {
		return nil
	}
	list := make([]k8sStorageClass, 0, len(payload.Items))
	for _, item := range payload.Items {
		def := item.Metadata.Annotations["storageclass.kubernetes.io/is-default-class"] == "true"
		list = append(list, k8sStorageClass{
			Name:              item.Metadata.Name,
			Provisioner:       item.Provisioner,
			ReclaimPolicy:     item.ReclaimPolicy,
			VolumeBindingMode: item.VolumeBindingMode,
			Default:           def,
		})
	}
	return list
}

func applyKubectlManifest(yaml string) (string, error) {
	tmp, err := os.CreateTemp("", "novasphere-sc-*.yaml")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.WriteString(yaml); err != nil {
		return "", err
	}
	tmp.Close()
	out, err := kubectlCmd("apply", "-f", tmp.Name()).CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// Host filesystem path as seen from the API pod (/host is the node root).
func hostPath(p string) string {
	p = normalizeHostPath(p)
	if strings.HasPrefix(p, "/host/") || p == "/host" {
		return p
	}
	return "/host" + p
}

// Map /host/bin|sbin paths to in-chroot absolute paths (/usr/bin, /usr/sbin).
func chrootCmdPath(bin string) string {
	if strings.HasPrefix(bin, "/host/bin/") {
		return "/usr/bin/" + strings.TrimPrefix(bin, "/host/bin/")
	}
	if strings.HasPrefix(bin, "/host/sbin/") {
		return "/usr/sbin/" + strings.TrimPrefix(bin, "/host/sbin/")
	}
	if strings.HasPrefix(bin, "/host/usr/bin/") {
		return "/usr/bin/" + strings.TrimPrefix(bin, "/host/usr/bin/")
	}
	if strings.HasPrefix(bin, "/host/usr/sbin/") {
		return "/usr/sbin/" + strings.TrimPrefix(bin, "/host/usr/sbin/")
	}
	return bin
}

func pathExists(p string) bool {
	hp := hostPath(p)
	if st, err := os.Stat(hp); err == nil {
		return st.IsDir() || !st.IsDir()
	}
	return false
}

func isHostMounted(mountPath string) bool {
	mp := normalizeHostPath(mountPath)
	for _, m := range readHostProcMounts() {
		if m == mp {
			return true
		}
	}
	return false
}

func nfsSource(server, export string) string {
	return strings.TrimSpace(server) + ":" + strings.TrimSpace(export)
}

func probeLocalPathStatus(mountPath string) (string, int64) {
	if !pathExists(mountPath) {
		return "unmounted", 0
	}
	if !isHostMounted(mountPath) {
		return "unmounted", 0
	}
	cmd := hostChrootCmd("df", "-B1", "--output=avail", normalizeHostPath(mountPath))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "mounted", 0
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) < 2 {
		return "mounted", 0
	}
	var avail int64
	fmt.Sscanf(strings.TrimSpace(lines[1]), "%d", &avail)
	return "mounted", avail
}

func probeNFSPathStatus(server, export, mountPath string) (string, int64) {
	if !isHostMounted(mountPath) {
		return "unmounted", 0
	}
	src := nfsSource(server, export)
	mounts := readHostProcMounts()
	mp := normalizeHostPath(mountPath)
	// readHostProcMounts maps device→mountpoint; check mountpoint match
	for dev, m := range mounts {
		if m == mp {
			if dev == src || strings.HasPrefix(dev, src) {
				return probeLocalPathStatus(mountPath)
			}
			return "mounted", 0
		}
	}
	return "unmounted", 0
}

func probeBackendStatus(b StorageBackend) (string, int64, string) {
	switch b.Type {
	case "local":
		mp, _ := b.Config["mount_path"].(string)
		st, cap := probeLocalPathStatus(mp)
		return st, cap, ""
	case "nfs":
		server, _ := b.Config["server"].(string)
		export, _ := b.Config["export_path"].(string)
		mp, _ := b.Config["mount_path"].(string)
		st, cap := probeNFSPathStatus(server, export, mp)
		return st, cap, ""
	case "ceph":
		// Ceph RBD is provisioned via CSI — no host mount to probe
		if b.Status == "error" {
			return "error", 0, b.Message
		}
		return "online", 0, ""
	default:
		return b.Status, b.CapacityBytes, b.Message
	}
}

// GET /api/v1/storage/classes — k8s StorageClasses + registered backends
func (h *StorageBackendsHandler) ListClasses(c *gin.Context) {
	cfg := loadStorageConfig()
	k8s := listK8sStorageClasses()

	classes := make([]gin.H, 0, len(k8s)+len(cfg.Backends))
	seen := map[string]bool{}
	for _, sc := range k8s {
		seen[sc.Name] = true
		classes = append(classes, gin.H{
			"id":               sc.Name,
			"name":             sc.Name,
			"provisioner":      sc.Provisioner,
			"status":           "active",
			"pool":             "—",
			"replication_factor": 1,
			"is_default":       sc.Default,
			"source":           "kubernetes",
		})
	}

	backends := make([]StorageBackend, 0, len(cfg.Backends))
	for _, b := range cfg.Backends {
		st, cap, _ := probeBackendStatus(b)
		b.Status = st
		b.CapacityBytes = cap
		backends = append(backends, b)
		if !seen[b.StorageClass] {
			classes = append(classes, gin.H{
				"id":                 b.ID,
				"name":               b.StorageClass,
				"provisioner":        backendProvisioner(b),
				"status":             st,
				"pool":               backendPool(b),
				"replication_factor": backendReplication(b),
				"type":               b.Type,
				"source":             "novasphere",
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"data":     classes,
		"items":    classes,
		"backends": backends,
		"total":    len(classes),
	})
}

func backendProvisioner(b StorageBackend) string {
	switch b.Type {
	case "nfs":
		return "k8s.io/nfs"
	case "ceph":
		return "rbd.csi.ceph.com"
	default:
		return "rancher.io/local-path"
	}
}

func backendPool(b StorageBackend) string {
	if b.Type == "ceph" {
		if p, ok := b.Config["pool"].(string); ok {
			return p
		}
	}
	return "—"
}

func backendReplication(b StorageBackend) int {
	if b.Type == "ceph" {
		if n, ok := b.Config["replication"].(float64); ok && n > 0 {
			return int(n)
		}
		return 3
	}
	return 1
}

type localStorageRequest struct {
	Name       string `json:"name" binding:"required"`
	Device     string `json:"device"`
	MountPath  string `json:"mount_path" binding:"required"`
	FSType     string `json:"fs_type"`
	Format     bool   `json:"format"`
	StorageClass string `json:"storage_class"`
}

// POST /api/v1/storage/local
func (h *StorageBackendsHandler) AddLocal(c *gin.Context) {
	var req localStorageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	scName := sanitizeStorageClass(firstNonEmpty(req.StorageClass, req.Name, "local-"+filepath.Base(req.MountPath)))
	if !safeNameRe.MatchString(scName) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid storage class name"})
		return
	}

	cfg := loadStorageConfig()
	if _, _, found := backendByID(cfg, scName); found {
		c.JSON(http.StatusConflict, gin.H{"error": "storage backend already registered", "storage_class": scName})
		return
	}

	fsType := firstNonEmpty(req.FSType, "xfs")
	backend := StorageBackend{
		ID:           uuid.New().String(),
		Type:         "local",
		Name:         firstNonEmpty(req.Name, scName),
		StorageClass: scName,
		Status:       "pending",
		CreatedAt:    time.Now().UTC(),
		Config: map[string]any{
			"device":      strings.TrimSpace(req.Device),
			"mount_path":  req.MountPath,
			"fs_type":     fsType,
			"format":      req.Format,
		},
	}

	msg, err := applyLocalBackend(backend)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	backend.Status, backend.CapacityBytes, _ = probeBackendStatus(backend)
	backend.Message = msg

	cfg.Backends = append(cfg.Backends, backend)
	if err := saveStorageConfig(cfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"backend": backend, "message": backend.Message})
}

func normalizeHostPath(p string) string {
	p = strings.TrimSpace(p)
	if strings.HasPrefix(p, "/host/") {
		return strings.TrimPrefix(p, "/host")
	}
	if p == "/host" {
		return "/"
	}
	return p
}

func readHostProcMounts() map[string]string {
	out := map[string]string{}
	b, err := os.ReadFile("/host/proc/mounts")
	if err != nil {
		b, err = os.ReadFile("/proc/mounts")
	}
	if err != nil {
		return out
	}
	for _, line := range strings.Split(string(b), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		out[fields[0]] = normalizeHostPath(fields[1])
	}
	return out
}

func deviceMountPoint(device string) string {
	device = strings.TrimSpace(device)
	if device == "" {
		return ""
	}
	mounts := readHostProcMounts()
	if mp, ok := mounts[device]; ok {
		return mp
	}
	// Resolve /dev/sdb → /dev/sdb1 etc. when the partition is mounted.
	for dev, mp := range mounts {
		if dev == device || strings.HasPrefix(dev, device) {
			return mp
		}
	}
	return ""
}

func applyLocalBackend(b StorageBackend) (string, error) {
	mountPath, _ := b.Config["mount_path"].(string)
	device, _ := b.Config["device"].(string)
	format, _ := b.Config["format"].(bool)
	fsType, _ := b.Config["fs_type"].(string)
	mountPath = strings.TrimSpace(mountPath)
	device = strings.TrimSpace(device)
	if fsType == "" {
		fsType = "xfs"
	}

	existingMP := deviceMountPoint(device)
	if device != "" && existingMP != "" {
		if mountPath != existingMP {
			return "", fmt.Errorf(
				"%s is already mounted at %s — use mount path %s and leave device empty, or unregister the existing local-path default",
				device, existingMP, existingMP,
			)
		}
		// Device already mounted at the requested path — register without remounting.
		device = ""
	}

	if device != "" && format {
		if out, err := hostChrootCmd("mkfs", "-t", fsType, device).CombinedOutput(); err != nil {
			return "", fmt.Errorf("format %s: %v (%s)", device, err, strings.TrimSpace(string(out)))
		}
	}
	if device != "" {
		if out, err := hostChrootCmd("mkdir", "-p", normalizeHostPath(mountPath)).CombinedOutput(); err != nil {
			return "", fmt.Errorf("prepare mount path %s: %v (%s)", mountPath, err, strings.TrimSpace(string(out)))
		}
		if out, err := hostChrootCmd("mount", device, normalizeHostPath(mountPath)).CombinedOutput(); err != nil {
			outStr := strings.TrimSpace(string(out))
			if !strings.Contains(outStr, "already mounted") {
				return "", fmt.Errorf("mount %s at %s: %v (%s)", device, mountPath, err, outStr)
			}
		}
	}
	if !pathExists(mountPath) {
		if device != "" {
			return "", fmt.Errorf("mount path not found after mount: %s", mountPath)
		}
		return "", fmt.Errorf("mount path not found on host: %s (register an existing mount or provide a device to mount)", mountPath)
	}
	if !isHostMounted(mountPath) && device == "" {
		return "", fmt.Errorf("path %s exists but is not mounted — use Mount action after registering, or provide device", mountPath)
	}

	yaml := fmt.Sprintf(`apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: %s
  labels:
    novasphere.vmalpha.com/backend-id: "%s"
    novasphere.vmalpha.com/type: local
provisioner: rancher.io/local-path
reclaimPolicy: Delete
volumeBindingMode: WaitForFirstConsumer
`, b.StorageClass, b.ID)

	out, err := applyKubectlManifest(yaml)
	if err != nil {
		return out, err
	}

	// Best-effort: add path to local-path ConfigMap when present
	patch := fmt.Sprintf(`{"data":{"config.json":"{\"paths\":[\"%s\"]}"}}`, strings.ReplaceAll(mountPath, `"`, `\"`))
	_, _ = kubectlCmd("-n", "local-path-storage", "patch", "configmap", "local-path-config", "--type", "merge", "-p", patch).CombinedOutput()

	return out, nil
}

type nfsStorageRequest struct {
	Name         string `json:"name" binding:"required"`
	Server       string `json:"server" binding:"required"`
	ExportPath   string `json:"export_path" binding:"required"`
	MountPath    string `json:"mount_path"`
	MountOptions string `json:"mount_options"`
	StorageClass string `json:"storage_class"`
}

// POST /api/v1/storage/nfs
func (h *StorageBackendsHandler) AddNFS(c *gin.Context) {
	var req nfsStorageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	scName := sanitizeStorageClass(firstNonEmpty(req.StorageClass, req.Name))
	cfg := loadStorageConfig()
	if _, _, found := backendByID(cfg, scName); found {
		c.JSON(http.StatusConflict, gin.H{"error": "storage backend already registered"})
		return
	}

	mountPath := firstNonEmpty(req.MountPath, filepath.Join("/var/lib/novasphere/nfs", scName))
	mountOpts := firstNonEmpty(req.MountOptions, "nfsvers=4.1")

	backend := StorageBackend{
		ID:           uuid.New().String(),
		Type:         "nfs",
		Name:         req.Name,
		StorageClass: scName,
		Status:       "pending",
		CreatedAt:    time.Now().UTC(),
		Config: map[string]any{
			"server":        req.Server,
			"export_path":   req.ExportPath,
			"mount_path":    mountPath,
			"mount_options": mountOpts,
		},
	}

	msg, err := applyNFSBackend(backend)
	if err != nil {
		backend.Status = "error"
		backend.Message = err.Error()
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "backend": backend})
		return
	}
	backend.Status, backend.CapacityBytes, _ = probeBackendStatus(backend)
	backend.Message = msg

	cfg.Backends = append(cfg.Backends, backend)
	if err := saveStorageConfig(cfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"backend": backend, "message": backend.Message})
}

func applyNFSBackend(b StorageBackend) (string, error) {
	server, _ := b.Config["server"].(string)
	export, _ := b.Config["export_path"].(string)
	mountPath, _ := b.Config["mount_path"].(string)
	mountOpts, _ := b.Config["mount_options"].(string)

	if err := ensureNFSMounted(server, export, mountPath, mountOpts); err != nil {
		return "", err
	}

	serverEsc := strings.ReplaceAll(server, `"`, `\"`)
	exportEsc := strings.ReplaceAll(export, `"`, `\"`)
	yaml := fmt.Sprintf(`apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: %s
  labels:
    novasphere.vmalpha.com/backend-id: "%s"
    novasphere.vmalpha.com/type: nfs
provisioner: kubernetes.io/no-provisioner
reclaimPolicy: Retain
volumeBindingMode: WaitForFirstConsumer
parameters:
  server: "%s"
  export: "%s"
---
apiVersion: v1
kind: PersistentVolume
metadata:
  name: %s-pv
  labels:
    novasphere.vmalpha.com/backend-id: "%s"
spec:
  capacity:
    storage: 100Gi
  accessModes:
    - ReadWriteMany
  persistentVolumeReclaimPolicy: Retain
  storageClassName: %s
  mountOptions:
    - %s
  nfs:
    server: %s
    path: %s
`, b.StorageClass, b.ID, serverEsc, exportEsc, b.StorageClass, b.ID, b.StorageClass, mountOpts, server, exportEsc)

	return applyKubectlManifest(yaml)
}

type cephStorageRequest struct {
	Name         string `json:"name" binding:"required"`
	Monitors     string `json:"monitors"`
	Pool         string `json:"pool" binding:"required"`
	User         string `json:"user"`
	Key          string `json:"key"`
	ClusterID    string `json:"cluster_id"`
	StorageClass string `json:"storage_class"`
	UseCluster   bool   `json:"use_cluster"`
}

// POST /api/v1/storage/ceph
func (h *StorageBackendsHandler) AddCeph(c *gin.Context) {
	var req cephStorageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	env := readClusterEnv()
	cephReady := false
	if out, err := exec.Command(hostBin("ceph"), "-s").CombinedOutput(); err == nil && len(out) > 0 {
		cephReady = true
	} else if env["ENABLE_CEPH"] == "1" {
		cephReady = true
	}

	externalCeph := strings.TrimSpace(req.Monitors) != "" && !req.UseCluster
	if !externalCeph {
		nodeCount := 0
		if out, err := kubectlCmd("get", "nodes", "--no-headers").CombinedOutput(); err == nil {
			nodeCount = len(strings.Fields(strings.TrimSpace(string(out))))
		}
		if !cephReady && nodeCount < 2 {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Ceph requires 2+ nodes — enable Ceph from Cluster setup first, or provide external monitor IPs",
				"nodes": nodeCount,
				"hint":  "/setup",
			})
			return
		}
	}

	scName := sanitizeStorageClass(firstNonEmpty(req.StorageClass, req.Name, "ceph-"+req.Pool))
	cfg := loadStorageConfig()
	if _, _, found := backendByID(cfg, scName); found {
		c.JSON(http.StatusConflict, gin.H{"error": "storage backend already registered"})
		return
	}

	mons := strings.TrimSpace(req.Monitors)
	if req.UseCluster && mons == "" {
		mons = discoverCephMonitors()
	}
	if mons == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "monitors required (comma-separated IPs or use_cluster)"})
		return
	}

	user := firstNonEmpty(req.User, "admin")
	pool := req.Pool
	clusterID := strings.TrimSpace(req.ClusterID)
	if clusterID == "" {
		clusterID = firstNonEmpty(os.Getenv("CEPH_CLUSTER_ID"), env["CEPH_CLUSTER_ID"])
	}
	if clusterID == "" {
		clusterID = discoverCephClusterID(mons)
	}
	if clusterID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "cluster_id required for external Ceph (Ceph FSID) — set in form or CEPH_CLUSTER_ID env",
			"hint":  "Run: ceph fsid on the Ceph cluster",
		})
		return
	}
	if strings.TrimSpace(req.Key) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Ceph user key is required for RBD CSI provisioning"})
		return
	}

	backend := StorageBackend{
		ID:           uuid.New().String(),
		Type:         "ceph",
		Name:         req.Name,
		StorageClass: scName,
		Status:       "pending",
		CreatedAt:    time.Now().UTC(),
		Config: map[string]any{
			"monitors":    mons,
			"pool":        pool,
			"user":        user,
			"cluster_id":  clusterID,
			"use_cluster": req.UseCluster,
			"replication": float64(3),
		},
	}
	backend.Config["has_key"] = true

	msg, err := applyCephBackend(backend, req.Key, clusterID)
	if err != nil {
		backend.Status = "error"
		backend.Message = err.Error()
	} else {
		backend.Status = "online"
		backend.Message = msg
	}

	cfg.Backends = append(cfg.Backends, backend)
	if err := saveStorageConfig(cfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"backend": backend, "message": backend.Message})
}

func discoverCephMonitors() string {
	out, err := exec.Command(hostBin("ceph"), "mon", "dump", "--format", "json").CombinedOutput()
	if err != nil {
		return ""
	}
	var payload struct {
		Mons []struct {
			Addr string `json:"addr"`
		} `json:"mons"`
	}
	if json.Unmarshal(out, &payload) != nil {
		return ""
	}
	var ips []string
	for _, m := range payload.Mons {
		addr := strings.Split(m.Addr, "/")[0]
		if addr != "" {
			ips = append(ips, addr)
		}
	}
	return strings.Join(ips, ",")
}

func discoverCephClusterID(monitors string) string {
	if fsid := strings.TrimSpace(os.Getenv("CEPH_FSID")); fsid != "" {
		return fsid
	}
	out, err := exec.Command(hostBin("ceph"), "fsid").CombinedOutput()
	if err == nil {
		return strings.TrimSpace(string(out))
	}
	// Try reading from host ceph.conf
	for _, p := range []string{"/etc/ceph/ceph.conf", "/host/etc/ceph/ceph.conf"} {
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(b), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "fsid") {
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					return parts[1]
				}
			}
		}
	}
	return ""
}

func applyCephBackend(b StorageBackend, key, clusterID string) (string, error) {
	mons, _ := b.Config["monitors"].(string)
	pool, _ := b.Config["pool"].(string)
	user, _ := b.Config["user"].(string)
	if clusterID == "" {
		clusterID = firstNonEmpty(os.Getenv("CEPH_CLUSTER_ID"), "ceph")
	}

	secretName := sanitizeStorageClass(b.StorageClass) + "-csi"
	secretYAML := fmt.Sprintf(`apiVersion: v1
kind: Secret
metadata:
  name: %s
  namespace: novasphere
type: kubernetes.io/rbd
stringData:
  userID: %s
  userKey: %s
`, secretName, user, key)
	if _, err := applyKubectlManifest(secretYAML); err != nil {
		return "", fmt.Errorf("create ceph secret: %w", err)
	}

	yaml := fmt.Sprintf(`apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: %s
  labels:
    novasphere.vmalpha.com/backend-id: "%s"
    novasphere.vmalpha.com/type: ceph
provisioner: rbd.csi.ceph.com
reclaimPolicy: Delete
allowVolumeExpansion: true
parameters:
  clusterID: "%s"
  pool: %s
  csi.storage.k8s.io/provisioner-secret-name: %s
  csi.storage.k8s.io/provisioner-secret-namespace: novasphere
  csi.storage.k8s.io/controller-expand-secret-name: %s
  csi.storage.k8s.io/controller-expand-secret-namespace: novasphere
  csi.storage.k8s.io/node-stage-secret-name: %s
  csi.storage.k8s.io/node-stage-secret-namespace: novasphere
`, b.StorageClass, b.ID, clusterID, pool, secretName, secretName, secretName)

	// Ceph CSI external cluster: create CephCluster CR or use monitors param depending on driver version.
	// For ceph-csi v3+, clusterID references a CephCluster CR; for lab we also pass monitors via ConfigMap.
	monYAML := fmt.Sprintf(`apiVersion: v1
kind: ConfigMap
metadata:
  name: ceph-csi-config
  namespace: novasphere
data:
  config.json: |
    [
      {
        "clusterID": "%s",
        "monitors": [%s],
        "cephFS": {},
        "rbd": {}
      }
    ]
`, clusterID, cephMonitorsJSON(mons))
	_, _ = applyKubectlManifest(monYAML)

	return applyKubectlManifest(yaml)
}

func cephMonitorsJSON(mons string) string {
	parts := strings.Split(mons, ",")
	var quoted []string
	for _, m := range parts {
		m = strings.TrimSpace(m)
		if m != "" {
			quoted = append(quoted, fmt.Sprintf(`"%s"`, m))
		}
	}
	return strings.Join(quoted, ",")
}

func ensureNFSMounted(server, export, mountPath, mountOpts string) error {
	mountPath = normalizeHostPath(mountPath)
	if isHostMounted(mountPath) {
		return nil
	}
	// Prefer writable pod mount at /var/lib/novasphere when path is under that tree
	if strings.HasPrefix(mountPath, "/var/lib/novasphere") {
		podPath := mountPath // same path — novasphere-lib volume is writable
		if err := os.MkdirAll(podPath, 0755); err != nil {
			return fmt.Errorf("prepare mount path %s: %v", mountPath, err)
		}
	} else if err := os.MkdirAll(hostPath(mountPath), 0755); err != nil {
		return fmt.Errorf("prepare mount path %s: %v", mountPath, err)
	}
	src := nfsSource(server, export)
	if out, err := hostChrootCmd("mount", "-t", "nfs", "-o", mountOpts, src, mountPath).CombinedOutput(); err != nil {
		outStr := strings.TrimSpace(string(out))
		if strings.Contains(outStr, "already mounted") {
			return nil
		}
		return fmt.Errorf("mount %s at %s: %v (%s)", src, mountPath, err, outStr)
	}
	return nil
}

func mountLocalBackend(b StorageBackend) error {
	mountPath, _ := b.Config["mount_path"].(string)
	device, _ := b.Config["device"].(string)
	fsType, _ := b.Config["fs_type"].(string)
	format, _ := b.Config["format"].(bool)
	mountPath = normalizeHostPath(mountPath)
	device = strings.TrimSpace(device)
	if fsType == "" {
		fsType = "xfs"
	}
	if isHostMounted(mountPath) {
		return nil
	}
	if device != "" {
		if format {
			if out, err := hostChrootCmd("mkfs", "-t", fsType, device).CombinedOutput(); err != nil {
				return fmt.Errorf("format %s: %v (%s)", device, err, strings.TrimSpace(string(out)))
			}
		}
		if out, err := hostChrootCmd("mkdir", "-p", mountPath).CombinedOutput(); err != nil {
			return fmt.Errorf("prepare mount path %s: %v (%s)", mountPath, err, strings.TrimSpace(string(out)))
		}
		if out, err := hostChrootCmd("mount", device, mountPath).CombinedOutput(); err != nil {
			outStr := strings.TrimSpace(string(out))
			if !strings.Contains(outStr, "already mounted") {
				return fmt.Errorf("mount %s at %s: %v (%s)", device, mountPath, err, outStr)
			}
		}
		return nil
	}
	if !pathExists(mountPath) {
		return fmt.Errorf("mount path not found on host: %s", mountPath)
	}
	return fmt.Errorf("path %s exists but is not mounted — provide device to mount", mountPath)
}

func unmountBackend(b StorageBackend) error {
	if b.Type == "ceph" {
		return fmt.Errorf("Ceph RBD backends are not host-mounted — unmount is not applicable")
	}
	mp, _ := b.Config["mount_path"].(string)
	mp = normalizeHostPath(mp)
	if mp == "" || !isHostMounted(mp) {
		return nil
	}
	out, err := kubectlCmd("get", "pvc", "-A", "-o", "json").CombinedOutput()
	if err == nil {
		var payload struct {
			Items []struct {
				Spec struct {
					StorageClassName string `json:"storageClassName"`
				} `json:"spec"`
				Status struct {
					Phase string `json:"phase"`
				} `json:"status"`
			} `json:"items"`
		}
		if json.Unmarshal(out, &payload) == nil {
			for _, pvc := range payload.Items {
				if pvc.Spec.StorageClassName == b.StorageClass && pvc.Status.Phase == "Bound" {
					return fmt.Errorf("cannot unmount: PVCs still bound to storage class %s", b.StorageClass)
				}
			}
		}
	}
	if out, err := hostChrootCmd("umount", mp).CombinedOutput(); err != nil {
		return fmt.Errorf("umount %s: %v (%s)", mp, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func updateBackendInConfig(id string, fn func(*StorageBackend)) (StorageBackend, error) {
	cfg := loadStorageConfig()
	b, idx, ok := backendByID(cfg, id)
	if !ok {
		return StorageBackend{}, fmt.Errorf("backend not found")
	}
	fn(&b)
	st, cap, _ := probeBackendStatus(b)
	b.Status = st
	b.CapacityBytes = cap
	cfg.Backends[idx] = b
	if err := saveStorageConfig(cfg); err != nil {
		return StorageBackend{}, err
	}
	return b, nil
}

// GET /api/v1/storage/backends/:id/status
func (h *StorageBackendsHandler) BackendStatus(c *gin.Context) {
	cfg := loadStorageConfig()
	b, _, ok := backendByID(cfg, c.Param("id"))
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "backend not found"})
		return
	}
	st, cap, msg := probeBackendStatus(b)
	c.JSON(http.StatusOK, gin.H{
		"id":             b.ID,
		"type":           b.Type,
		"storage_class":  b.StorageClass,
		"status":         st,
		"capacity_bytes": cap,
		"message":        firstNonEmpty(msg, b.Message),
		"mounted":        st == "mounted" || st == "online",
	})
}

// POST /api/v1/storage/backends/:id/mount
func (h *StorageBackendsHandler) MountBackend(c *gin.Context) {
	id := c.Param("id")
	cfg := loadStorageConfig()
	b, _, ok := backendByID(cfg, id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "backend not found"})
		return
	}
	var err error
	switch b.Type {
	case "local":
		err = mountLocalBackend(b)
	case "nfs":
		server, _ := b.Config["server"].(string)
		export, _ := b.Config["export_path"].(string)
		mp, _ := b.Config["mount_path"].(string)
		opts, _ := b.Config["mount_options"].(string)
		err = ensureNFSMounted(server, export, mp, opts)
	case "ceph":
		c.JSON(http.StatusBadRequest, gin.H{"error": "Ceph RBD is provisioned via CSI — no host mount needed"})
		return
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported backend type"})
		return
	}
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	updated, err := updateBackendInConfig(id, func(b *StorageBackend) {
		b.Message = "mounted"
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"backend": updated, "message": "mounted"})
}

// POST /api/v1/storage/backends/:id/unmount
func (h *StorageBackendsHandler) UnmountBackend(c *gin.Context) {
	id := c.Param("id")
	cfg := loadStorageConfig()
	b, _, ok := backendByID(cfg, id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "backend not found"})
		return
	}
	if err := unmountBackend(b); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	updated, err := updateBackendInConfig(id, func(b *StorageBackend) {
		b.Message = "unmounted"
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"backend": updated, "message": "unmounted"})
}

// DELETE /api/v1/storage/backends/:id — remove registration (does not delete data)
func (h *StorageBackendsHandler) DeleteBackend(c *gin.Context) {
	id := c.Param("id")
	confirm := c.Query("confirm") == "true" || c.Query("confirm") == "1"
	if !confirm {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "confirmation required",
			"message": "Pass ?confirm=true to unregister backend (Kubernetes StorageClass/PV are not deleted)",
		})
		return
	}

	cfg := loadStorageConfig()
	b, idx, ok := backendByID(cfg, id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "backend not found"})
		return
	}
	cfg.Backends = append(cfg.Backends[:idx], cfg.Backends[idx+1:]...)
	if err := saveStorageConfig(cfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"removed":        b.ID,
		"storage_class":  b.StorageClass,
		"data_preserved": true,
		"message":        "Backend unregistered — existing PVCs and data were not deleted",
	})
}
