package handlers

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/novasphere/novasphere/internal/config"
	"github.com/novasphere/novasphere/internal/services/nativehost"
)

func nativeTestBackend(verifiedAt string) StorageBackend {
	return StorageBackend{
		Type: "ceph", Status: "online", Message: "verified",
		Config: map[string]any{
			"managed_by": "ceph-registration", "native_managed": true,
			"verified_at": verifiedAt, "pool": "vmalpha-rbd",
		},
	}
}

func TestNativeBackendStatusRequiresFreshVerification(t *testing.T) {
	fresh := nativeTestBackend(time.Now().UTC().Format(time.RFC3339))
	status, _, _ := probeBackendStatus(fresh)
	if status != "online" {
		t.Fatalf("fresh native backend status = %q, want online", status)
	}

	stale := nativeTestBackend(time.Now().UTC().Add(-nativeBackendVerificationTTL - time.Second).Format(time.RFC3339))
	status, _, message := probeBackendStatus(stale)
	if status != "unknown" || message == "" {
		t.Fatalf("stale native backend = (%q, %q), want unknown with explanation", status, message)
	}
}

func TestNativeBackendsUseNativeProvisioner(t *testing.T) {
	rbd := nativeTestBackend(time.Now().UTC().Format(time.RFC3339))
	if got := backendProvisioner(rbd); got != "vmalpha.io/native-rbd" {
		t.Fatalf("RBD provisioner = %q", got)
	}
	nfs := rbd
	nfs.Type = "nfs"
	nfs.Config["cephfs"] = map[string]any{"data_pool": "vmalpha-cephfs-data"}
	if got := backendProvisioner(nfs); got != "vmalpha.io/native-ceph-nfs" {
		t.Fatalf("NFS provisioner = %q", got)
	}
	if got := backendPool(nfs); got != "vmalpha-cephfs-data" {
		t.Fatalf("NFS data pool = %q", got)
	}
}

func TestRefreshNativeRegistrationPersistsOnlyFreshBrokerProof(t *testing.T) {
	oldPath := storageConfigPath
	storageConfigPath = filepath.Join(t.TempDir(), "storage.json")
	t.Cleanup(func() { storageConfigPath = oldPath })
	now := time.Now().UTC().Format(time.RFC3339)
	manifest := map[string]any{
		"version": 1, "status": "verified", "verified_at": now, "fsid": "test-fsid",
		"monitors":        []string{"192.168.71.81:6789"},
		"placement_nodes": []string{"kvm11.vmalpha.com", "kvm12.vmalpha.com", "kvm13.vmalpha.com"},
		"checks":          map[string]bool{"rbd_read_write": true, "cephfs_read_write": true, "nfs_read_write": true},
		"host_access": map[string]map[string]bool{
			"kvm11.vmalpha.com": {"rbd_read_write": true, "nfs_read_write": true},
			"kvm12.vmalpha.com": {"rbd_read_write": true, "nfs_read_write": true},
			"kvm13.vmalpha.com": {"rbd_read_write": true, "nfs_read_write": true},
		},
		"rbd_pool": "vmalpha-rbd",
		"cephfs":   map[string]string{"name": "vmalpha-fs", "data_pool": "vmalpha-cephfs-data", "metadata_pool": "vmalpha-cephfs-metadata"},
		"nfs":      map[string]string{"service": "vmalpha-nfs", "export": "/vmalpha", "endpoint": "192.168.71.84:2049"},
	}
	raw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	native := NewNativeHandler(nil, map[string]config.NativeHostConfig{"kvm11": {Address: "192.0.2.11:22"}})
	native.Connect = func(config.NativeHostConfig) (nativeBroker, error) {
		return registrationBroker{raw: raw}, nil
	}
	h := NewStorageBackendsHandler(nil, nil)
	h.Native = native
	if err := h.RefreshNativeRegistration(context.Background(), "kvm11"); err != nil {
		t.Fatal(err)
	}
	cfg := loadStorageConfig()
	if len(cfg.Backends) != 2 || cfg.Backends[0].Name != "VM Alpha Storage" || cfg.Backends[1].Name != "VM Alpha NFS" {
		t.Fatalf("unexpected registered backends: %#v", cfg.Backends)
	}
	if cfg.Backends[1].Config["endpoint"] != "192.168.71.84:2049" {
		t.Fatal("stable endpoint was not persisted")
	}

	manifest["verified_at"] = time.Now().UTC().Add(-nativeBackendVerificationTTL - time.Minute).Format(time.RFC3339)
	stale, _ := json.Marshal(manifest)
	native.Connect = func(config.NativeHostConfig) (nativeBroker, error) { return registrationBroker{raw: stale}, nil }
	if err := h.RefreshNativeRegistration(context.Background(), "kvm11"); err == nil {
		t.Fatal("accepted stale proof")
	}
	if got := loadStorageConfig(); got.Backends[0].Config["verified_at"] != now {
		t.Fatal("stale refresh overwrote prior proof")
	}
}

type registrationBroker struct{ raw json.RawMessage }

func (b registrationBroker) Call(_ context.Context, request nativehost.Request) (json.RawMessage, error) {
	if request.Operation != "ceph.telemetry" || request.Arguments["query"] != "storage registration" {
		return nil, context.Canceled
	}
	return b.raw, nil
}
