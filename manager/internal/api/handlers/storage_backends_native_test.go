package handlers

import (
	"testing"
	"time"
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
