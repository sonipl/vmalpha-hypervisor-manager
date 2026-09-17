package handlers

import (
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/novasphere/novasphere/internal/models"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// CephMetricsHandler serves live + historical Ceph cluster telemetry for the Storage UI.
type CephMetricsHandler struct {
	DB     *gorm.DB
	Log    *logrus.Logger
	once   sync.Once
	stopCh chan struct{}
}

func NewCephMetricsHandler(db *gorm.DB, log *logrus.Logger) *CephMetricsHandler {
	h := &CephMetricsHandler{DB: db, Log: log, stopCh: make(chan struct{})}
	h.once.Do(func() { go h.samplerLoop() })
	return h
}

func cephBin(name string) string {
	for _, p := range []string{
		"/host/sbin/" + name,
		"/host/bin/" + name,
		"/usr/sbin/" + name,
		"/usr/bin/" + name,
	} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return name
}

func cephCacheDir() string {
	for _, d := range []string{"/var/lib/novasphere/ceph", "/host/var/lib/novasphere/ceph"} {
		if st, err := os.Stat(d); err == nil && st.IsDir() {
			return d
		}
	}
	return ""
}

func readCephCache(name string) ([]byte, error) {
	dir := cephCacheDir()
	if dir == "" {
		return nil, os.ErrNotExist
	}
	return os.ReadFile(dir + "/" + name)
}

// runCeph prefers host-side JSON samples (Alpine API cannot exec glibc cephadm),
// then falls back to ceph/cephadm CLI when available.
func runCeph(args ...string) ([]byte, error) {
	joined := strings.Join(args, " ")
	switch {
	case strings.HasPrefix(joined, "-s") || strings.Contains(joined, "status"):
		if b, err := readCephCache("status.json"); err == nil && len(b) > 0 {
			return b, nil
		}
	case strings.HasPrefix(joined, "df"):
		if b, err := readCephCache("df.json"); err == nil && len(b) > 0 {
			return b, nil
		}
	case strings.Contains(joined, "osd perf"):
		if b, err := readCephCache("osd_perf.json"); err == nil && len(b) > 0 {
			return b, nil
		}
	case strings.Contains(joined, "orch host ls"):
		if b, err := readCephCache("hosts.json"); err == nil && len(b) > 0 {
			return b, nil
		}
	case strings.Contains(joined, "osd tree"):
		if b, err := readCephCache("osd_tree.json"); err == nil && len(b) > 0 {
			return b, nil
		}
	}

	env := append(os.Environ(),
		"KUBECONFIG=/etc/kubernetes/admin.conf",
		"PATH=/usr/sbin:/usr/bin:/sbin:/bin",
	)
	cmd := exec.Command(cephBin("ceph"), args...)
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if err == nil {
		return out, nil
	}
	cargs := append([]string{"shell", "--", "ceph"}, args...)
	cmd2 := exec.Command(cephBin("cephadm"), cargs...)
	cmd2.Env = env
	return cmd2.CombinedOutput()
}

type cephStatusJSON struct {
	Health struct {
		Status string `json:"status"`
	} `json:"health"`
	OSDMap struct {
		NumOSDs   int `json:"num_osds"`
		NumUpOSDs int `json:"num_up_osds"`
		NumInOSDs int `json:"num_in_osds"`
	} `json:"osdmap"`
	PGMap struct {
		BytesUsed     int64   `json:"bytes_used"`
		BytesAvail    int64   `json:"bytes_avail"`
		BytesTotal    int64   `json:"bytes_total"`
		ReadBytesSec  float64 `json:"read_bytes_sec"`
		WriteBytesSec float64 `json:"write_bytes_sec"`
		ReadOpPerSec  float64 `json:"read_op_per_sec"`
		WriteOpPerSec float64 `json:"write_op_per_sec"`
	} `json:"pgmap"`
}

type cephDFJSON struct {
	Stats struct {
		TotalBytes      int64 `json:"total_bytes"`
		TotalUsedBytes  int64 `json:"total_used_bytes"`
		TotalAvailBytes int64 `json:"total_avail_bytes"`
	} `json:"stats"`
}

func (h *CephMetricsHandler) sampleOnce() {
	out, err := runCeph("-s", "--format", "json")
	if err != nil {
		h.Log.Debugf("ceph sample skipped: %v (%s)", err, strings.TrimSpace(string(out)))
		return
	}
	var st cephStatusJSON
	if json.Unmarshal(out, &st) != nil {
		return
	}
	sample := models.CephMetricSample{
		CreatedAt:  time.Now().UTC(),
		Health:     st.Health.Status,
		TotalBytes: st.PGMap.BytesTotal,
		UsedBytes:  st.PGMap.BytesUsed,
		AvailBytes: st.PGMap.BytesAvail,
		NumOSD:     st.OSDMap.NumOSDs,
		NumOSDUp:   st.OSDMap.NumUpOSDs,
		NumOSDIn:   st.OSDMap.NumInOSDs,
		ReadIOPS:   st.PGMap.ReadOpPerSec,
		WriteIOPS:  st.PGMap.WriteOpPerSec,
		RawJSON:    string(out),
	}
	if dfOut, err := runCeph("df", "--format", "json"); err == nil {
		var df cephDFJSON
		if json.Unmarshal(dfOut, &df) == nil && df.Stats.TotalBytes > 0 {
			sample.TotalBytes = df.Stats.TotalBytes
			sample.UsedBytes = df.Stats.TotalUsedBytes
			sample.AvailBytes = df.Stats.TotalAvailBytes
		}
	}
	if perfOut, err := runCeph("osd", "perf", "--format", "json"); err == nil {
		var perf struct {
			OSDPerfInfos []struct {
				PerfStats struct {
					ApplyLatencyMs  float64 `json:"apply_latency_ms"`
					CommitLatencyMs float64 `json:"commit_latency_ms"`
				} `json:"perf_stats"`
			} `json:"osd_perf_infos"`
		}
		if json.Unmarshal(perfOut, &perf) == nil && len(perf.OSDPerfInfos) > 0 {
			var sumA, sumC float64
			for _, p := range perf.OSDPerfInfos {
				sumA += p.PerfStats.ApplyLatencyMs
				sumC += p.PerfStats.CommitLatencyMs
			}
			n := float64(len(perf.OSDPerfInfos))
			sample.WriteLatencyMs = sumA / n
			sample.ReadLatencyMs = sumC / n
		}
	}
	if hostsOut, err := runCeph("orch", "host", "ls", "--format", "json"); err == nil {
		var hosts []any
		if json.Unmarshal(hostsOut, &hosts) == nil {
			sample.NumHosts = len(hosts)
		}
	}
	if err := h.DB.Create(&sample).Error; err != nil {
		h.Log.Warnf("ceph sample persist: %v", err)
	}
	cutoff := time.Now().UTC().Add(-7 * 24 * time.Hour)
	h.DB.Where("created_at < ?", cutoff).Delete(&models.CephMetricSample{})
}

func (h *CephMetricsHandler) samplerLoop() {
	t := time.NewTicker(15 * time.Second)
	defer t.Stop()
	h.sampleOnce()
	for {
		select {
		case <-t.C:
			h.sampleOnce()
		case <-h.stopCh:
			return
		}
	}
}

// GET /api/v1/storage/ceph/health
func (h *CephMetricsHandler) GetHealth(c *gin.Context) {
	out, err := runCeph("-s", "--format", "json")
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"available": false,
			"health":    "UNKNOWN",
			"error":     strings.TrimSpace(string(out)),
		})
		return
	}
	var raw map[string]any
	_ = json.Unmarshal(out, &raw)
	health := "UNKNOWN"
	if hs, ok := raw["health"].(map[string]any); ok {
		if s, ok := hs["status"].(string); ok {
			health = s
		}
	}
	dfOut, _ := runCeph("df", "--format", "json")
	var df any
	_ = json.Unmarshal(dfOut, &df)
	hostsOut, _ := runCeph("orch", "host", "ls", "--format", "json")
	var hosts any
	_ = json.Unmarshal(hostsOut, &hosts)
	osdOut, _ := runCeph("osd", "tree", "--format", "json")
	var osdTree any
	_ = json.Unmarshal(osdOut, &osdTree)

	c.JSON(http.StatusOK, gin.H{
		"available":  true,
		"health":     health,
		"status":     raw,
		"df":         df,
		"hosts":      hosts,
		"osd_tree":   osdTree,
		"checked_at": time.Now().UTC(),
	})
}

// GET /api/v1/storage/ceph/metrics?range=1h|6h|24h|7d
func (h *CephMetricsHandler) GetMetrics(c *gin.Context) {
	rng := c.DefaultQuery("range", "1h")
	dur := time.Hour
	switch rng {
	case "15m":
		dur = 15 * time.Minute
	case "6h":
		dur = 6 * time.Hour
	case "24h":
		dur = 24 * time.Hour
	case "7d":
		dur = 7 * 24 * time.Hour
	}
	since := time.Now().UTC().Add(-dur)
	var samples []models.CephMetricSample
	h.DB.Where("created_at >= ?", since).Order("created_at asc").Find(&samples)

	live := gin.H{"available": false}
	if out, err := runCeph("-s", "--format", "json"); err == nil {
		var st cephStatusJSON
		if json.Unmarshal(out, &st) == nil {
			live = gin.H{
				"available":       true,
				"health":          st.Health.Status,
				"total_bytes":     st.PGMap.BytesTotal,
				"used_bytes":      st.PGMap.BytesUsed,
				"avail_bytes":     st.PGMap.BytesAvail,
				"num_osd":         st.OSDMap.NumOSDs,
				"num_osd_up":      st.OSDMap.NumUpOSDs,
				"num_osd_in":      st.OSDMap.NumInOSDs,
				"read_iops":       st.PGMap.ReadOpPerSec,
				"write_iops":      st.PGMap.WriteOpPerSec,
				"read_bytes_sec":  st.PGMap.ReadBytesSec,
				"write_bytes_sec": st.PGMap.WriteBytesSec,
			}
		}
	}

	series := make([]gin.H, 0, len(samples))
	for _, s := range samples {
		series = append(series, gin.H{
			"t":                s.CreatedAt,
			"health":           s.Health,
			"used_bytes":       s.UsedBytes,
			"total_bytes":      s.TotalBytes,
			"avail_bytes":      s.AvailBytes,
			"read_iops":        s.ReadIOPS,
			"write_iops":       s.WriteIOPS,
			"read_latency_ms":  s.ReadLatencyMs,
			"write_latency_ms": s.WriteLatencyMs,
			"num_osd_up":       s.NumOSDUp,
			"num_hosts":        s.NumHosts,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"range":             rng,
		"since":             since,
		"live":              live,
		"series":            series,
		"sample_count":      len(series),
		"interval_hint_sec": 15,
	})
}
