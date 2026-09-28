package handlers

import (
	"context"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/novasphere/novasphere/internal/config"
	"github.com/novasphere/novasphere/internal/services/nativehost"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestBrowserRejectsTraversal(t *testing.T) {
	for _, p := range []string{"/etc/passwd", "../secret", "a/../../secret", "a\\b", "a\x00b"} {
		if _, e := browserPath(p); e == nil {
			t.Fatalf("accepted %q", p)
		}
	}
	if p, e := browserPath("folder/file.iso"); e != nil || p != "folder/file.iso" {
		t.Fatal(p, e)
	}
}
func TestBrowserMountTypes(t *testing.T) {
	ext := StorageBackend{Type: "nfs", Config: map[string]any{"external_native": true, "datastore_id": "external-nfs", "server": "192.168.71.50", "export_path": "/nfs"}}
	p, s, v, e := browserMountConfig(ext)
	if e != nil || p != externalMountBase+"external-nfs" || s != "192.168.71.50:/nfs" || v != "3" {
		t.Fatal(p, s, v, e)
	}
	ceph := StorageBackend{Type: "nfs", Config: map[string]any{"native_managed": true, "managed_by": "ceph-registration", "service": "vmalpha-nfs", "endpoint": "192.168.71.84:2049", "export_path": "/vmalpha"}}
	p, s, v, e = browserMountConfig(ceph)
	if e != nil || p != externalMountBase+"ceph-vmalpha-nfs" || s != "192.168.71.84:/vmalpha" || v != "4.1" {
		t.Fatal(p, s, v, e)
	}
	ceph.Type = "ceph"
	if _, _, _, e = browserMountConfig(ceph); e == nil {
		t.Fatal("RBD browsing accepted")
	}
}

type nfsEvidenceBroker struct{ source string }

func (f nfsEvidenceBroker) Call(_ context.Context, r nativehost.Request) (json.RawMessage, error) {
	b, _ := json.Marshal(map[string]any{"source": f.source, "mount_path": externalMountBase + "external-nfs", "automount": true, "options": []string{"vers=3", "soft"}, "observed_at": time.Now().UTC()})
	return b, nil
}
func TestExternalRegistrationPreservesCephAndRejectsMismatch(t *testing.T) {
	old := storageConfigPath
	storageConfigPath = t.TempDir() + "/storage.json"
	defer func() { storageConfigPath = old }()
	if e := saveStorageConfig(storageConfigFile{Backends: []StorageBackend{{ID: "ceph-existing", Type: "ceph", Name: "VM Alpha Storage"}}}); e != nil {
		t.Fatal(e)
	}
	n := NewNativeHandler(nil, map[string]config.NativeHostConfig{"kvm11": {}})
	source := "wrong:/nfs"
	n.Connect = func(config.NativeHostConfig) (nativeBroker, error) { return nfsEvidenceBroker{source}, nil }
	h := &StorageBackendsHandler{Native: n}
	call := func() int {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/", strings.NewReader(`{"id":"external-nfs","name":"External NFS","server":"192.168.71.50","export_path":"/nfs","hosts":["kvm11"]}`))
		c.Request.Header.Set("Content-Type", "application/json")
		h.AddExternalNFS(c)
		return w.Code
	}
	if code := call(); code != 409 {
		t.Fatal(code)
	}
	if len(loadStorageConfig().Backends) != 1 {
		t.Fatal("changed storage on failure")
	}
	source = "192.168.71.50:/nfs"
	if code := call(); code != 201 {
		t.Fatal(code)
	}
	if code := call(); code != 200 {
		t.Fatal(code)
	}
	cfg := loadStorageConfig()
	if len(cfg.Backends) != 2 || cfg.Backends[0].ID != "ceph-existing" {
		t.Fatal("Ceph overwritten or duplicate created")
	}
}
