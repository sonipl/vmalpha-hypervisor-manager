package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"time"
)

const nativeRegistrationRefreshInterval = 5 * time.Minute

type nativeRegistrationManifest struct {
	Version        int      `json:"version"`
	Status         string   `json:"status"`
	VerifiedAt     string   `json:"verified_at"`
	FSID           string   `json:"fsid"`
	Monitors       []string `json:"monitors"`
	PlacementNodes []string `json:"placement_nodes"`
	Checks         struct {
		RBD    bool `json:"rbd_read_write"`
		CephFS bool `json:"cephfs_read_write"`
		NFS    bool `json:"nfs_read_write"`
	} `json:"checks"`
	HostAccess map[string]struct {
		RBD bool `json:"rbd_read_write"`
		NFS bool `json:"nfs_read_write"`
	} `json:"host_access"`
	RBDPool string `json:"rbd_pool"`
	CephFS  struct {
		Name         string `json:"name"`
		DataPool     string `json:"data_pool"`
		MetadataPool string `json:"metadata_pool"`
	} `json:"cephfs"`
	NFS struct {
		Service  string `json:"service"`
		Export   string `json:"export"`
		Endpoint string `json:"endpoint"`
	} `json:"nfs"`
}

func nativeText(value, name string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("missing %s", name)
	}
	return value, nil
}

func validateNativeRegistration(raw json.RawMessage, now time.Time) (nativeRegistrationManifest, error) {
	var manifest nativeRegistrationManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return manifest, fmt.Errorf("invalid storage registration manifest")
	}
	if manifest.Version != 1 || manifest.Status != "verified" {
		return manifest, fmt.Errorf("storage registration is not verified")
	}
	verifiedAt, err := time.Parse(time.RFC3339, manifest.VerifiedAt)
	if err != nil {
		return manifest, fmt.Errorf("invalid storage registration verification timestamp")
	}
	if now.Sub(verifiedAt) > nativeBackendVerificationTTL || verifiedAt.After(now.Add(time.Minute)) {
		return manifest, fmt.Errorf("storage registration verification is stale")
	}
	for name, value := range map[string]string{
		"fsid": manifest.FSID, "rbd_pool": manifest.RBDPool, "cephfs.name": manifest.CephFS.Name,
		"cephfs.data_pool": manifest.CephFS.DataPool, "cephfs.metadata_pool": manifest.CephFS.MetadataPool,
		"nfs.service": manifest.NFS.Service, "nfs.export": manifest.NFS.Export, "nfs.endpoint": manifest.NFS.Endpoint,
	} {
		if _, err := nativeText(value, name); err != nil {
			return manifest, err
		}
	}
	if _, _, err := net.SplitHostPort(manifest.NFS.Endpoint); err != nil {
		return manifest, fmt.Errorf("nfs.endpoint must be host:port")
	}
	if len(manifest.Monitors) == 0 || len(manifest.PlacementNodes) == 0 || !manifest.Checks.RBD || !manifest.Checks.CephFS || !manifest.Checks.NFS {
		return manifest, fmt.Errorf("storage registration is missing verified storage checks")
	}
	for _, host := range []string{"kvm11.vmalpha.com", "kvm12.vmalpha.com", "kvm13.vmalpha.com"} {
		evidence, ok := manifest.HostAccess[host]
		if !ok || !evidence.RBD || !evidence.NFS {
			return manifest, fmt.Errorf("storage registration lacks RBD/NFS evidence for %s", host)
		}
	}
	return manifest, nil
}

func nativeBackends(manifest nativeRegistrationManifest, now time.Time) []StorageBackend {
	common := map[string]any{
		"managed_by": "ceph-registration", "native_managed": true, "fsid": manifest.FSID,
		"monitors": manifest.Monitors, "verified_at": manifest.VerifiedAt,
		"placement_nodes": manifest.PlacementNodes, "host_access": manifest.HostAccess,
	}
	rbdConfig := map[string]any{}
	for key, value := range common {
		rbdConfig[key] = value
	}
	rbdConfig["pool"], rbdConfig["access"] = manifest.RBDPool, "native-rbd"
	nfsConfig := map[string]any{}
	for key, value := range common {
		nfsConfig[key] = value
	}
	nfsConfig["service"], nfsConfig["export_path"], nfsConfig["endpoint"], nfsConfig["access"] = manifest.NFS.Service, manifest.NFS.Export, manifest.NFS.Endpoint, "native-nfs"
	nfsConfig["cephfs"] = map[string]any{"name": manifest.CephFS.Name, "data_pool": manifest.CephFS.DataPool, "metadata_pool": manifest.CephFS.MetadataPool}
	return []StorageBackend{
		{ID: "ceph-rbd-" + manifest.FSID + "-" + manifest.RBDPool, Type: "ceph", Name: "VM Alpha Storage", StorageClass: "native-rbd", Status: "online", Message: "Agent-verified native RBD read/write", CreatedAt: now, Config: rbdConfig},
		{ID: "ceph-nfs-" + manifest.FSID + "-" + manifest.NFS.Service, Type: "nfs", Name: "VM Alpha NFS", StorageClass: "native-ceph-nfs", Status: "online", Message: "Agent-verified Ceph-backed NFS read/write", CreatedAt: now, Config: nfsConfig},
	}
}

// RefreshNativeRegistration obtains a fresh proof through the pinned native
// broker. It never changes the Ceph-supplied verification time, and leaves the
// prior state untouched if the broker or manifest validation fails.
func (h *StorageBackendsHandler) RefreshNativeRegistration(ctx context.Context, host string) error {
	if h.Native == nil {
		return fmt.Errorf("native host enrollment is unavailable")
	}
	raw, err := h.Native.StorageRegistration(ctx, host)
	if err != nil {
		return err
	}
	manifest, err := validateNativeRegistration(raw, time.Now().UTC())
	if err != nil {
		return err
	}
	backends := nativeBackends(manifest, time.Now().UTC())
	storageRegistrationMu.Lock()
	defer storageRegistrationMu.Unlock()
	cfg := loadStorageConfig()
	preserved := make([]StorageBackend, 0, len(cfg.Backends))
	for _, backend := range cfg.Backends {
		if nativeManagedBackend(backend) && backend.Config["fsid"] == manifest.FSID {
			continue
		}
		preserved = append(preserved, backend)
	}
	cfg.Backends = append(preserved, backends...)
	return saveStorageConfig(cfg)
}

// StartNativeRegistrationRefresh launches a bounded read-only reconciliation.
// Failed refreshes are logged and preserve the last genuine verification; its
// timestamp then makes the displayed backend state become unknown.
func (h *StorageBackendsHandler) StartNativeRegistrationRefresh(host string) {
	if h.Native == nil || !h.Native.IsEnrolled(host) {
		return
	}
	refresh := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		if err := h.RefreshNativeRegistration(ctx, host); err != nil && h.Log != nil {
			h.Log.WithError(err).Warn("native Ceph storage registration refresh skipped")
		}
	}
	refresh()
	go func() {
		ticker := time.NewTicker(nativeRegistrationRefreshInterval)
		defer ticker.Stop()
		for range ticker.C {
			refresh()
		}
	}()
}
