package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/novasphere/novasphere/internal/services/nativehost"
	"net"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"
)

var storageRegistrationMu sync.Mutex

var externalID = regexp.MustCompile(`^[a-z][a-z0-9-]{0,40}$`)
var externalServer = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9.-]{0,252}$`)
var externalExport = regexp.MustCompile(`^/[A-Za-z0-9_./-]*$`)

const externalMountBase = "/var/lib/vmalpha/datastores/"

type externalNFSRequest struct {
	ID     string   `json:"id"`
	Name   string   `json:"name"`
	Server string   `json:"server"`
	Export string   `json:"export_path"`
	Hosts  []string `json:"hosts"`
}

func (r externalNFSRequest) valid() bool {
	if !externalID.MatchString(r.ID) || !externalServer.MatchString(r.Server) || !externalExport.MatchString(r.Export) || strings.TrimSpace(r.Name) == "" || len(r.Name) > 128 || len(r.Hosts) == 0 || len(r.Hosts) > 128 {
		return false
	}
	for _, p := range strings.Split(r.Export, "/") {
		if p == ".." {
			return false
		}
	}
	seen := map[string]bool{}
	for _, h := range r.Hosts {
		if h == "" || seen[h] {
			return false
		}
		seen[h] = true
	}
	return true
}
func externalNativeBackend(b StorageBackend) bool {
	v, _ := b.Config["external_native"].(bool)
	return b.Type == "nfs" && v
}
func browserMountConfig(b StorageBackend) (string, string, string, error) {
	if b.Type != "nfs" {
		return "", "", "", fmt.Errorf("RBD and non-NFS browsing unsupported")
	}
	export, _ := b.Config["export_path"].(string)
	if !externalExport.MatchString(export) {
		return "", "", "", fmt.Errorf("invalid registered export")
	}
	if externalNativeBackend(b) {
		id, _ := b.Config["datastore_id"].(string)
		server, _ := b.Config["server"].(string)
		if externalID.MatchString(id) && externalServer.MatchString(server) {
			return externalMountBase + id, server + ":" + export, "3", nil
		}
	}
	if nativeManagedBackend(b) {
		service, _ := b.Config["service"].(string)
		endpoint, _ := b.Config["endpoint"].(string)
		server, port, err := net.SplitHostPort(endpoint)
		if err == nil && port == "2049" && externalID.MatchString(service) && externalServer.MatchString(server) {
			return externalMountBase + "ceph-" + service, server + ":" + export, "4.1", nil
		}
	}
	return "", "", "", fmt.Errorf("NFS browser configuration unavailable")
}
func externalMountIdentity(ctx context.Context, b StorageBackend) error {
	target, source, version, err := browserMountConfig(b)
	if err != nil {
		return err
	}
	out, err := exec.CommandContext(ctx, "findmnt", "-J", "-M", target, "-o", "SOURCE,FSTYPE,OPTIONS").Output()
	if err != nil {
		return fmt.Errorf("NFS is not mounted on Manager")
	}
	var data struct {
		Filesystems []struct {
			Source  string `json:"source"`
			FSType  string `json:"fstype"`
			Options string `json:"options"`
		} `json:"filesystems"`
	}
	if json.Unmarshal(out, &data) != nil {
		return fmt.Errorf("mount unavailable")
	}
	// systemd automount exposes an autofs entry and the real NFS entry at the
	// same target. Only the latter carries the identity we need to validate.
	var m struct {
		Source  string `json:"source"`
		FSType  string `json:"fstype"`
		Options string `json:"options"`
	}
	found := false
	for _, candidate := range data.Filesystems {
		if candidate.FSType == "nfs" || candidate.FSType == "nfs4" {
			m, found = candidate, true
			break
		}
	}
	if !found {
		return fmt.Errorf("mount unavailable")
	}
	opts := "," + m.Options + ","
	if m.Source != source || (m.FSType != "nfs" && m.FSType != "nfs4") || !strings.Contains(opts, ",vers="+version+",") || !strings.Contains(opts, ",soft,") {
		return fmt.Errorf("mount identity or NFS options differ")
	}
	unit, err := exec.CommandContext(ctx, "systemd-escape", "--path", "--suffix=automount", target).Output()
	if err != nil {
		return fmt.Errorf("automount unit unavailable")
	}
	state, err := exec.CommandContext(ctx, "systemctl", "is-active", strings.TrimSpace(string(unit))).Output()
	if err != nil || strings.TrimSpace(string(state)) != "active" {
		return fmt.Errorf("automount inactive")
	}
	return nil
}

func (h *StorageBackendsHandler) AddExternalNFS(c *gin.Context) {
	var r externalNFSRequest
	if c.ShouldBindJSON(&r) != nil || !r.valid() {
		c.JSON(400, gin.H{"error": "valid datastore id, name, server, export_path and distinct enrolled hosts required"})
		return
	}
	if h.Native == nil {
		c.JSON(503, gin.H{"error": "native enrollment unavailable"})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 45*time.Second)
	defer cancel()
	for _, name := range r.Hosts {
		cfg, ok := h.Native.Hosts[name]
		if !ok {
			c.JSON(400, gin.H{"error": "host not enrolled", "host": name})
			return
		}
		broker, err := h.Native.Connect(cfg)
		if err != nil {
			c.JSON(503, gin.H{"error": "host connection unavailable", "host": name})
			return
		}
		raw, err := broker.Call(ctx, nativehost.Request{Operation: "storage.external-nfs.status", Arguments: map[string]any{"id": r.ID}})
		var evidence struct {
			Source     string    `json:"source"`
			MountPath  string    `json:"mount_path"`
			Automount  bool      `json:"automount"`
			Options    []string  `json:"options"`
			ObservedAt time.Time `json:"observed_at"`
		}
		if err != nil || json.Unmarshal(raw, &evidence) != nil || evidence.Source != r.Server+":"+r.Export || evidence.MountPath != externalMountBase+r.ID || !evidence.Automount || time.Since(evidence.ObservedAt) > time.Minute || time.Until(evidence.ObservedAt) > time.Minute || !containsOption(evidence.Options, "vers=3") || !containsOption(evidence.Options, "soft") {
			c.JSON(409, gin.H{"error": "host NFSv3 soft automount not verified", "host": name})
			return
		}
	}
	storageRegistrationMu.Lock()
	defer storageRegistrationMu.Unlock()
	cfg := loadStorageConfig()
	id := "external-nfs-" + r.ID
	for i, b := range cfg.Backends {
		if b.ID == id && externalNativeBackend(b) && b.Config["server"] == r.Server && b.Config["export_path"] == r.Export {
			b.Name = r.Name
			b.Config["hosts"] = r.Hosts
			b.Config["verified_at"] = time.Now().UTC().Format(time.RFC3339)
			b.Status = "online"
			cfg.Backends[i] = b
			if saveStorageConfig(cfg) != nil {
				c.JSON(500, gin.H{"error": "cannot save datastore"})
				return
			}
			c.JSON(200, b)
			return
		}
		if b.ID == id || b.Name == r.Name {
			c.JSON(409, gin.H{"error": "datastore already registered; existing backend preserved"})
			return
		}
	}
	b := StorageBackend{ID: id, Type: "nfs", Name: r.Name, StorageClass: id, Status: "online", CreatedAt: time.Now().UTC(), Config: map[string]any{"external_native": true, "datastore_id": r.ID, "server": r.Server, "export_path": r.Export, "mount_path": externalMountBase + r.ID, "hosts": r.Hosts, "verified_at": time.Now().UTC().Format(time.RFC3339), "mount_options": "vers=3,soft,nofail,_netdev,x-systemd.automount,x-systemd.mount-timeout=30s"}}
	cfg.Backends = append(cfg.Backends, b)
	if saveStorageConfig(cfg) != nil {
		c.JSON(500, gin.H{"error": "cannot save datastore"})
		return
	}
	c.JSON(201, b)
}
func containsOption(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}
func externalBackendStatus(b StorageBackend) (string, int64, string) {
	s, _ := b.Config["verified_at"].(string)
	t, err := time.Parse(time.RFC3339, s)
	if err != nil || time.Since(t) > nativeBackendVerificationTTL {
		return "unknown", 0, "Host mount verification expired; revalidate external NFS"
	}
	return "online", 0, "External NFS mounts verified on selected hypervisors"
}
