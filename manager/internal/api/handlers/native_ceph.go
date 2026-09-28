package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/novasphere/novasphere/internal/services/nativehost"
	"strings"
	"time"
)

// No shell fragments, credentials or stale cache are accepted by this collector.
func (h *CephMetricsHandler) cephQuery(ctx context.Context, query string) ([]byte, error) {
	if h.native == nil {
		return nil, errors.New("Ceph host telemetry unavailable")
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	for _, cfg := range h.native.Hosts {
		broker, err := h.native.Connect(cfg)
		if err != nil {
			continue
		}
		raw, err := broker.Call(ctx, nativehost.Request{Operation: "ceph.telemetry", Arguments: map[string]any{"query": query}})
		if err == nil && json.Valid(raw) {
			return raw, nil
		}
		if ctx.Err() != nil {
			break
		}
	}
	return nil, errors.New("Ceph host telemetry unavailable")
}
func (h *CephMetricsHandler) runCeph(args ...string) ([]byte, error) {
	query := strings.TrimSuffix(strings.Join(args, " "), " --format json")
	if query == "-s" {
		query = "status"
	}
	return h.cephQuery(context.Background(), query)
}
func (h *CephMetricsHandler) HealthStatus(ctx context.Context) (string, error) {
	raw, err := h.cephQuery(ctx, "status")
	if err != nil {
		return "", err
	}
	var st cephStatusJSON
	if json.Unmarshal(raw, &st) != nil {
		return "", errors.New("invalid Ceph status")
	}
	switch st.Health.Status {
	case "HEALTH_OK", "HEALTH_WARN", "HEALTH_ERR":
		return st.Health.Status, nil
	}
	return "", errors.New("Ceph health unavailable")
}

func validCephHealth(status string) bool {
	return status == "HEALTH_OK" || status == "HEALTH_WARN" || status == "HEALTH_ERR"
}
