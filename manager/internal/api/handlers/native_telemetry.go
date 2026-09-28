package handlers

import (
	"context"
	"encoding/json"
	"math"
	"sync"
	"time"

	"github.com/novasphere/novasphere/internal/services/nativehost"
)

type hostMeasurement struct {
	CPU      int      `json:"cpu"`
	CPUUsage *float64 `json:"cpuUsage"`
	Memory   struct {
		Total float64 `json:"total"`
		Used  float64 `json:"used"`
	} `json:"memory"`
	Storage struct {
		Total float64 `json:"total"`
		Used  float64 `json:"used"`
	} `json:"storage"`
}

// Collect fresh samples only. Partial coverage must never look like complete
// cluster utilization, and root filesystem usage is not Ceph capacity.
func (h *NativeHandler) dashboardTelemetry(ctx context.Context) map[string]any {
	utilization := map[string]any{"cpu_percent": nil, "memory_percent": nil, "storage_percent": nil, "network_mbps": nil}
	result := map[string]any{"telemetry_status": "unavailable", "utilization": utilization, "hosts_expected": len(h.Hosts), "hosts_observed": 0, "storage_scope": "host_root_filesystems"}
	if len(h.Hosts) == 0 {
		return result
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	samples := make(chan hostMeasurement, len(h.Hosts))
	var wg sync.WaitGroup
	for _, cfg := range h.Hosts {
		wg.Add(1)
		go func() {
			defer wg.Done()
			broker, err := h.Connect(cfg)
			if err != nil {
				return
			}
			raw, err := broker.Call(ctx, nativehost.Request{Operation: "inventory", Arguments: map[string]any{}})
			if err != nil {
				return
			}
			var sample hostMeasurement
			if json.Unmarshal(raw, &sample) != nil || !sample.valid() {
				return
			}
			samples <- sample
		}()
	}
	wg.Wait()
	close(samples)
	var cpu, cores, memoryUsed, memoryTotal, storageUsed, storageTotal float64
	observed := 0
	for s := range samples {
		observed++
		cores += float64(s.CPU)
		cpu += *s.CPUUsage * float64(s.CPU)
		memoryUsed += s.Memory.Used
		memoryTotal += s.Memory.Total
		storageUsed += s.Storage.Used
		storageTotal += s.Storage.Total
	}
	result["hosts_observed"] = observed
	if observed != len(h.Hosts) {
		if observed > 0 {
			result["telemetry_status"] = "partial"
		}
		return result
	}
	utilization["cpu_percent"] = cpu / cores
	utilization["memory_percent"] = 100 * memoryUsed / memoryTotal
	utilization["storage_percent"] = 100 * storageUsed / storageTotal
	result["telemetry_status"] = "live"
	result["observed_at"] = time.Now().UTC()
	return result
}

func (s hostMeasurement) valid() bool {
	if s.CPU <= 0 || s.CPUUsage == nil {
		return false
	}
	for _, v := range []float64{*s.CPUUsage, s.Memory.Total, s.Memory.Used, s.Storage.Total, s.Storage.Used} {
		if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
			return false
		}
	}
	return *s.CPUUsage <= 100 && s.Memory.Total > 0 && s.Memory.Used <= s.Memory.Total && s.Storage.Total > 0 && s.Storage.Used <= s.Storage.Total
}
