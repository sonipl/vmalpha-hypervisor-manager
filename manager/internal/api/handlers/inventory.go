package handlers

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/novasphere/novasphere/internal/models"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

func ensureDefaultClusterID(db *gorm.DB) (uuid.UUID, error) {
	var cl models.Cluster
	if err := db.Where("name = ?", "default").First(&cl).Error; err == nil {
		return cl.ID, nil
	}
	cl = models.Cluster{
		BaseModel:   models.BaseModel{ID: uuid.New()},
		Name:        "default",
		Description: "vmalpha-os.10 day-2 cluster",
	}
	if err := db.Create(&cl).Error; err != nil {
		return uuid.Nil, err
	}
	return cl.ID, nil
}

func parseK8sMemoryMi(s string) int {
	s = strings.TrimSpace(s)
	if strings.HasSuffix(s, "Ki") {
		v, _ := strconv.Atoi(strings.TrimSuffix(s, "Ki"))
		return v / 1024
	}
	if strings.HasSuffix(s, "Mi") {
		v, _ := strconv.Atoi(strings.TrimSuffix(s, "Mi"))
		return v
	}
	if strings.HasSuffix(s, "Gi") {
		v, _ := strconv.Atoi(strings.TrimSuffix(s, "Gi"))
		return v * 1024
	}
	return 0
}

// syncHostsFromKubernetes upserts k8s nodes into the hosts table.
func syncHostsFromKubernetes(db *gorm.DB, log *logrus.Logger) {
	out, err := kubectlCmd("get", "nodes", "-o", "json").CombinedOutput()
	if err != nil {
		if log != nil {
			log.Warnf("inventory: kubectl get nodes: %v (%s)", err, strings.TrimSpace(string(out)))
		}
		return
	}

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
				Addresses []struct {
					Type    string `json:"type"`
					Address string `json:"address"`
				} `json:"addresses"`
				NodeInfo struct {
					KernelVersion  string `json:"kernelVersion"`
					OSImage        string `json:"osImage"`
					KubeletVersion string `json:"kubeletVersion"`
				} `json:"nodeInfo"`
				Capacity map[string]string `json:"capacity"`
			} `json:"status"`
		} `json:"items"`
	}
	if json.Unmarshal(out, &payload) != nil {
		return
	}

	clusterID, err := ensureDefaultClusterID(db)
	if err != nil {
		if log != nil {
			log.Warnf("inventory: default cluster: %v", err)
		}
		return
	}

	for _, n := range payload.Items {
		ready := false
		for _, cond := range n.Status.Conditions {
			if cond.Type == "Ready" && cond.Status == "True" {
				ready = true
				break
			}
		}
		ip := ""
		for _, addr := range n.Status.Addresses {
			if addr.Type == "InternalIP" {
				ip = addr.Address
				break
			}
		}
		cpuCores := 0
		if v, ok := n.Status.Capacity["cpu"]; ok {
			cpuCores, _ = strconv.Atoi(v)
		}
		memMB := 0
		if v, ok := n.Status.Capacity["memory"]; ok {
			memMB = parseK8sMemoryMi(v)
		}

		status := models.HostStatusReady
		if !ready {
			status = models.HostStatusError
		}

		labels := models.JSONMap{
			"kubelet_version": n.Status.NodeInfo.KubeletVersion,
			"source":          "kubernetes",
		}
		for k, v := range n.Metadata.Labels {
			if strings.HasPrefix(k, "node-role.kubernetes.io/") {
				labels["role"] = strings.TrimPrefix(k, "node-role.kubernetes.io/")
				labels["role_value"] = v
			}
		}

		var existing models.Host
		err := db.Where("name = ?", n.Metadata.Name).First(&existing).Error
		if err == gorm.ErrRecordNotFound {
			host := models.Host{
				BaseModel: models.BaseModel{ID: uuid.New()},
				Name:      n.Metadata.Name,
				Status:    status,
				IPAddress: ip,
				ClusterID: clusterID,
				CPUCores:  cpuCores,
				MemoryMB:  memMB,
				KernelVer: n.Status.NodeInfo.KernelVersion,
				OSVersion: n.Status.NodeInfo.OSImage,
				Labels:    labels,
			}
			if err := db.Create(&host).Error; err != nil && log != nil {
				log.Warnf("inventory: create host %s: %v", n.Metadata.Name, err)
			}
		} else if err == nil {
			db.Model(&existing).Updates(map[string]interface{}{
				"status":         status,
				"ip_address":     ip,
				"cpu_cores":      cpuCores,
				"memory_mb":      memMB,
				"kernel_version": n.Status.NodeInfo.KernelVersion,
				"os_version":     n.Status.NodeInfo.OSImage,
				"labels":         labels,
			})
		}
	}
}

// syncVMsFromKubeVirt upserts KubeVirt VMs into the database when available.
func syncVMsFromKubeVirt(db *gorm.DB, log *logrus.Logger) {
	nsOut, err := kubectlCmd("get", "ns", "kubevirt", "-o", "name").CombinedOutput()
	if err != nil || !strings.Contains(string(nsOut), "kubevirt") {
		return
	}

	out, err := kubectlCmd("get", "virtualmachines", "-A", "-o", "json").CombinedOutput()
	if err != nil {
		if log != nil {
			log.Debugf("inventory: kubectl get virtualmachines: %v", err)
		}
		return
	}

	var payload struct {
		Items []struct {
			Metadata struct {
				Name      string `json:"name"`
				Namespace string `json:"namespace"`
			} `json:"metadata"`
			Status struct {
				PrintableStatus string `json:"printableStatus"`
			} `json:"status"`
			Spec struct {
				Template struct {
					Spec struct {
						Domain struct {
							CPU struct {
								Cores   int `json:"cores"`
								Sockets int `json:"sockets"`
								Threads int `json:"threads"`
							} `json:"cpu"`
							Resources struct {
								Requests map[string]string `json:"requests"`
							} `json:"resources"`
						} `json:"domain"`
						NodeSelector map[string]string `json:"nodeSelector"`
					} `json:"spec"`
				} `json:"template"`
			} `json:"spec"`
		} `json:"items"`
	}
	if json.Unmarshal(out, &payload) != nil {
		return
	}

	// Map running instances to nodes
	nodeByVM := map[string]string{}
	if vmiOut, err := kubectlCmd("get", "virtualmachineinstances", "-A", "-o", "json").CombinedOutput(); err == nil {
		var vmiPayload struct {
			Items []struct {
				Metadata struct {
					Name      string `json:"name"`
					Namespace string `json:"namespace"`
				} `json:"metadata"`
				Status struct {
					NodeName string `json:"nodeName"`
					Phase    string `json:"phase"`
				} `json:"status"`
			} `json:"items"`
		}
		if json.Unmarshal(vmiOut, &vmiPayload) == nil {
			for _, vmi := range vmiPayload.Items {
				nodeByVM[vmi.Metadata.Namespace+"/"+vmi.Metadata.Name] = vmi.Status.NodeName
			}
		}
	}

	for _, vm := range payload.Items {
		key := vm.Metadata.Namespace + "/" + vm.Metadata.Name
		status := mapKubeVirtStatus(vm.Status.PrintableStatus)
		vcpus := vm.Spec.Template.Spec.Domain.CPU.Cores
		if vcpus == 0 {
			vcpus = 1
		}
		if vm.Spec.Template.Spec.Domain.CPU.Sockets > 0 {
			vcpus = vm.Spec.Template.Spec.Domain.CPU.Sockets * max(1, vm.Spec.Template.Spec.Domain.CPU.Cores) * max(1, vm.Spec.Template.Spec.Domain.CPU.Threads)
		}
		memMB := 0
		if v, ok := vm.Spec.Template.Spec.Domain.Resources.Requests["memory"]; ok {
			memMB = parseK8sMemoryMi(v)
		}

		var existing models.VirtualMachine
		err := db.Where("namespace = ? AND name = ?", vm.Metadata.Namespace, vm.Metadata.Name).First(&existing).Error
		if err == gorm.ErrRecordNotFound {
			rec := models.VirtualMachine{
				BaseModel: models.BaseModel{ID: uuid.New()},
				Name:      vm.Metadata.Name,
				Namespace: vm.Metadata.Namespace,
				Status:    status,
				VCPUs:     vcpus,
				MemoryMB:  memMB,
				HostNode:  nodeByVM[key],
			}
			if err := db.Create(&rec).Error; err != nil && log != nil {
				log.Warnf("inventory: create vm %s: %v", key, err)
			}
		} else if err == nil {
			db.Model(&existing).Updates(map[string]interface{}{
				"status":    status,
				"vcpus":     vcpus,
				"memory_mb": memMB,
				"host_node": nodeByVM[key],
			})
		}
	}
}

func mapKubeVirtStatus(s string) models.VMStatus {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "running":
		return models.VMStatusRunning
	case "stopped", "halted":
		return models.VMStatusStopped
	case "paused":
		return models.VMStatusPaused
	case "migrating":
		return models.VMStatusMigrating
	case "provisioning", "starting":
		return models.VMStatusProvisioning
	default:
		if s == "" {
			return models.VMStatusStopped
		}
		return models.VMStatusError
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
