package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/novasphere/novasphere/internal/models"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type SetupHandler struct {
	DB  *gorm.DB
	Log *logrus.Logger
}

func NewSetupHandler(db *gorm.DB, log *logrus.Logger) *SetupHandler {
	return &SetupHandler{DB: db, Log: log}
}

type setupNodeRequest struct {
	Name     string `json:"name" binding:"required"`
	IP       string `json:"ip" binding:"required"`
	Role     string `json:"role" binding:"required"` // worker | control-plane | ceph
	Hostname string `json:"hostname"`
}

func hostBin(name string) string {
	for _, dir := range []string{"/host/bin", "/host/sbin", "/host/usr/bin", "/host/usr/sbin"} {
		p := dir + "/" + name
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return name
}

// Run a command in the host mount namespace (privileged pod with hostPID).
func hostChrootCmd(args ...string) *exec.Cmd {
	if len(args) == 0 {
		args = []string{"true"}
	}
	cmdInChroot := chrootCmdPath(hostBin(args[0]))
	// nsenter into PID 1 (host init) mount namespace so mounts persist on the node
	argv := append([]string{"nsenter", "-t", "1", "-m", "-u", "-i", "-n", "--", cmdInChroot}, args[1:]...)
	if _, err := exec.LookPath("nsenter"); err != nil {
		argv = append([]string{"chroot", "/host", cmdInChroot}, args[1:]...)
	}
	return exec.Command(argv[0], argv[1:]...)
}

func kubeconfigPath() string {
	if v := strings.TrimSpace(os.Getenv("KUBECONFIG")); v != "" {
		return v
	}
	for _, p := range []string{"/etc/kubernetes/admin.conf", "/var/lib/vmalpha-os/kube/admin.conf"} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return "/etc/kubernetes/admin.conf"
}

func kubectlCmd(args ...string) *exec.Cmd {
	cmd := exec.Command(hostBin("kubectl"), args...)
	cmd.Env = append(os.Environ(), "KUBECONFIG="+kubeconfigPath())
	return cmd
}

// GET /api/v1/setup/status
func (h *SetupHandler) GetStatus(c *gin.Context) {
	clusterEnv := readClusterEnv()
	status := gin.H{
		"kubernetes": gin.H{"available": false, "nodes": []any{}},
		"kubevirt":   gin.H{"available": false},
		"ceph":       gin.H{"available": false, "summary": ""},
		"novasphere": gin.H{"ui": "ok"},
		"cluster": gin.H{
			"size":           clusterEnv["CLUSTER_SIZE"],
			"storage_class":  clusterEnv["STORAGE_CLASS"],
			"ceph_enabled":   clusterEnv["ENABLE_CEPH"] == "1",
			"ceph_network":   clusterEnv["CEPH_NETWORK"],
			"can_enable_ceph": false,
		},
	}

	out, err := kubectlCmd("get", "nodes", "-o", "json").CombinedOutput()
	if err == nil {
		var payload struct {
			Items []struct {
				Metadata struct {
					Name   string            `json:"name"`
					Labels map[string]string `json:"labels"`
				} `json:"metadata"`
				Status struct {
					Conditions []struct {
						Type   string `json:"type"`
						Status string `json:"status"`
					} `json:"conditions"`
					NodeInfo struct {
						KubeletVersion string `json:"kubeletVersion"`
					} `json:"nodeInfo"`
				} `json:"status"`
			} `json:"items"`
		}
		if json.Unmarshal(out, &payload) == nil {
			list := make([]gin.H, 0, len(payload.Items))
			readyCount := 0
			for _, n := range payload.Items {
				ready := false
				for _, cond := range n.Status.Conditions {
					if cond.Type == "Ready" && cond.Status == "True" {
						ready = true
						readyCount++
						break
					}
				}
				list = append(list, gin.H{
					"name":    n.Metadata.Name,
					"ready":   ready,
					"roles":   n.Metadata.Labels,
					"version": n.Status.NodeInfo.KubeletVersion,
				})
			}
			status["kubernetes"] = gin.H{
				"available": true,
				"nodes":     list,
				"ready":     readyCount,
				"total":     len(list),
			}
		}
	} else {
		h.Log.Warnf("kubectl get nodes: %v (%s)", err, strings.TrimSpace(string(out)))
	}

	if out, err := kubectlCmd("get", "ns", "kubevirt", "-o", "name").CombinedOutput(); err == nil && strings.Contains(string(out), "kubevirt") {
		status["kubevirt"] = gin.H{"available": true, "namespace": "kubevirt"}
	}

	if out, err := exec.Command(hostBin("ceph"), "-s", "--format", "json").CombinedOutput(); err == nil {
		status["ceph"] = gin.H{"available": true, "summary": string(out)}
	} else if out, err := exec.Command(hostBin("cephadm"), "shell", "--", "ceph", "-s").CombinedOutput(); err == nil {
		status["ceph"] = gin.H{"available": true, "summary": string(out)}
	} else {
		status["ceph"] = gin.H{"available": false, "summary": "ceph not ready on this node"}
	}

	var hosts []models.Host
	_ = h.DB.Order("created_at desc").Limit(50).Find(&hosts).Error
	status["registered_hosts"] = hosts
	status["checked_at"] = time.Now().UTC()

	if k8s, ok := status["kubernetes"].(gin.H); ok {
		if total, ok := k8s["total"].(int); ok && total >= 2 {
			if cl, ok := status["cluster"].(gin.H); ok {
				cl["can_enable_ceph"] = clusterEnv["ENABLE_CEPH"] != "1"
			}
		}
	}

	c.JSON(http.StatusOK, status)
}

// POST /api/v1/setup/nodes
func (h *SetupHandler) AddNode(c *gin.Context) {
	var req setupNodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	role := strings.ToLower(req.Role)
	if role != "worker" && role != "control-plane" && role != "ceph" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "role must be worker, control-plane, or ceph"})
		return
	}

	clusterID, err := h.ensureDefaultCluster()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	host := models.Host{
		BaseModel: models.BaseModel{ID: uuid.New()},
		Name:      req.Name,
		Status:    models.HostStatusReady,
		IPAddress: req.IP,
		ClusterID: clusterID,
		Labels: models.JSONMap{
			"role":     role,
			"hostname": req.Hostname,
			"setup":    "pending",
		},
	}
	if err := h.DB.Create(&host).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	resp := gin.H{"host": host, "role": role}
	switch role {
	case "worker", "control-plane":
		joinCmd, err := h.kubeadmJoinCommand(role == "control-plane")
		if err != nil {
			h.Log.Warnf("join command: %v", err)
			resp["join_command"] = ""
			resp["join_hint"] = "Run on control-plane: kubeadm token create --print-join-command"
			resp["warning"] = err.Error()
		} else {
			resp["join_command"] = joinCmd
			resp["join_hint"] = fmt.Sprintf("SSH to %s as root and run join_command (host must already have vmalpha-os.10).", req.IP)
		}
	case "ceph":
		resp["ceph_command"] = fmt.Sprintf("ceph orch host add %s %s", firstNonEmpty(req.Hostname, req.Name), req.IP)
		resp["join_hint"] = "On Ceph admin host run ceph_command after cephadm bootstrap."
	}
	c.JSON(http.StatusCreated, resp)
}

// POST /api/v1/setup/nodes/:id/join-k8s
func (h *SetupHandler) JoinK8s(c *gin.Context) {
	id := c.Param("id")
	var host models.Host
	if err := h.DB.First(&host, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "host not found"})
		return
	}
	role, _ := host.Labels["role"].(string)
	joinCmd, err := h.kubeadmJoinCommand(role == "control-plane")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"host_id": host.ID, "join_command": joinCmd})
}

// POST /api/v1/setup/ceph/hosts
func (h *SetupHandler) AddCephHost(c *gin.Context) {
	var req struct {
		Name     string `json:"name" binding:"required"`
		IP       string `json:"ip" binding:"required"`
		Hostname string `json:"hostname"`
		Devices  string `json:"devices"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	hn := firstNonEmpty(req.Hostname, req.Name)
	cmdStr := fmt.Sprintf("ceph orch host add %s %s", hn, req.IP)
	applied := false
	var output string
	if out, err := exec.Command("bash", "-lc", cmdStr).CombinedOutput(); err == nil {
		applied = true
		output = string(out)
	} else {
		output = string(out)
		if strings.TrimSpace(output) == "" && err != nil {
			output = err.Error()
		}
	}
	if req.Devices != "" && applied {
		if out, err := exec.Command("bash", "-lc", "ceph orch apply osd --all-available-devices").CombinedOutput(); err == nil {
			output += "\n" + string(out)
		}
	}

	clusterID, _ := h.ensureDefaultCluster()
	host := models.Host{
		BaseModel: models.BaseModel{ID: uuid.New()},
		Name:      req.Name,
		IPAddress: req.IP,
		Status:    models.HostStatusReady,
		ClusterID: clusterID,
		Labels:    models.JSONMap{"role": "ceph", "hostname": hn, "setup": "ceph"},
	}
	_ = h.DB.Create(&host).Error

	c.JSON(http.StatusCreated, gin.H{
		"host":         host,
		"ceph_command": cmdStr,
		"applied":      applied,
		"output":       output,
	})
}

func (h *SetupHandler) kubeadmJoinCommand(controlPlane bool) (string, error) {
	args := []string{"token", "create", "--print-join-command"}
	cmd := exec.Command(hostBin("kubeadm"), args...)
	cmd.Env = append(os.Environ(), "KUBECONFIG="+kubeconfigPath())
	out, err := cmd.CombinedOutput()
	if err == nil {
		line := strings.TrimSpace(string(out))
		for _, l := range strings.Split(line, "\n") {
			l = strings.TrimSpace(l)
			if strings.HasPrefix(l, "kubeadm join") {
				line = l
				break
			}
		}
		if controlPlane && !strings.Contains(line, "--control-plane") {
			line += " --control-plane"
		}
		return line, nil
	}
	for _, p := range []string{"/tmp/join-command.sh", "/etc/vmalpha-os/join-command.sh"} {
		if b, err2 := os.ReadFile(p); err2 == nil {
			for _, l := range strings.Split(string(b), "\n") {
				l = strings.TrimSpace(l)
				if strings.HasPrefix(l, "kubeadm join") {
					if controlPlane && !strings.Contains(l, "--control-plane") {
						l += " --control-plane"
					}
					return l, nil
				}
			}
		}
	}
	return "", fmt.Errorf("kubeadm join unavailable: %v (%s)", err, strings.TrimSpace(string(out)))
}


func (h *SetupHandler) ensureDefaultCluster() (uuid.UUID, error) {
	var cl models.Cluster
	if err := h.DB.Where("name = ?", "default").First(&cl).Error; err == nil {
		return cl.ID, nil
	}
	cl = models.Cluster{
		BaseModel:   models.BaseModel{ID: uuid.New()},
		Name:        "default",
		Description: "vmalpha-os.10 day-2 cluster",
	}
	if err := h.DB.Create(&cl).Error; err != nil {
		return uuid.Nil, err
	}
	return cl.ID, nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func readClusterEnv() map[string]string {
	out := map[string]string{
		"CLUSTER_SIZE":  "1",
		"STORAGE_CLASS": "local-path",
		"ENABLE_CEPH":   "0",
	}
	b, err := os.ReadFile("/etc/vmalpha-os/cluster.env")
	if err != nil {
		if b2, err2 := os.ReadFile("/host/etc/vmalpha-os/cluster.env"); err2 == nil {
			b = b2
		} else {
			return out
		}
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if k, v, ok := strings.Cut(line, "="); ok {
			out[k] = v
		}
	}
	return out
}

// POST /api/v1/setup/ceph/enable — bootstrap Ceph when cluster has 2+ nodes
func (h *SetupHandler) EnableCeph(c *gin.Context) {
	env := readClusterEnv()
	if env["ENABLE_CEPH"] == "1" {
		c.JSON(http.StatusConflict, gin.H{"error": "Ceph already enabled"})
		return
	}
	out, err := kubectlCmd("get", "nodes", "--no-headers").CombinedOutput()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "kubernetes not ready", "detail": strings.TrimSpace(string(out))})
		return
	}
	nodeCount := len(strings.Fields(strings.TrimSpace(string(out))))
	if nodeCount < 2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "need at least 2 nodes before enabling Ceph", "nodes": nodeCount})
		return
	}
	// Mark want-ceph and trigger bootstrap on host
	flagDir := "/host/etc/vmalpha-os/flags"
	if _, err := os.Stat(flagDir); err != nil {
		flagDir = "/etc/vmalpha-os/flags"
	}
	_ = os.MkdirAll(flagDir, 0755)
	_ = os.WriteFile(filepath.Join(flagDir, "want-ceph"), []byte("1\n"), 0644)
	cmd := exec.Command(hostBin("systemctl"), "start", "vmalpha-ceph-bootstrap.service")
	if out, err := cmd.CombinedOutput(); err != nil {
		h.Log.Warnf("ceph bootstrap start: %v (%s)", err, strings.TrimSpace(string(out)))
	}
	c.JSON(http.StatusAccepted, gin.H{
		"message":      "Ceph bootstrap started — monitor progress in Storage UI",
		"nodes":        nodeCount,
		"ceph_network": env["CEPH_NETWORK"],
	})
}

// DELETE /api/v1/setup/nodes/:id — remove registered host (DB); drain K8s node when present
func (h *SetupHandler) RemoveNode(c *gin.Context) {
	id := c.Param("id")
	var host models.Host
	if err := h.DB.First(&host, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "host not found"})
		return
	}
	nodeName := firstNonEmpty(host.Name, "")
	if hn, ok := host.Labels["hostname"].(string); ok && hn != "" {
		nodeName = hn
	}
	drained := false
	if nodeName != "" {
		if out, err := kubectlCmd("get", "node", nodeName).CombinedOutput(); err == nil && len(out) > 0 {
			_, _ = kubectlCmd("cordon", nodeName).CombinedOutput()
			_, _ = kubectlCmd("drain", nodeName, "--ignore-daemonsets", "--delete-emptydir-data", "--force", "--grace-period=60").CombinedOutput()
			_, _ = kubectlCmd("delete", "node", nodeName).CombinedOutput()
			drained = true
		}
	}
	if err := h.DB.Delete(&host).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"removed": host.ID, "name": host.Name, "drained": drained})
}
