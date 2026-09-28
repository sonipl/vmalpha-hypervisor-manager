package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/novasphere/novasphere/internal/config"
	"github.com/novasphere/novasphere/internal/services/nativehost"
)

type sampleBroker struct{ raw string }

func (b sampleBroker) Call(context.Context, nativehost.Request) (json.RawMessage, error) {
	return json.RawMessage(b.raw), nil
}

func TestTelemetryCoverageAndWeightedSamples(t *testing.T) {
	h := NewNativeHandler(nil, map[string]config.NativeHostConfig{"one": {Address: "one"}, "two": {Address: "two"}})
	h.Connect = func(c config.NativeHostConfig) (nativeBroker, error) {
		if c.Address == "one" {
			return sampleBroker{`{"cpu":2,"cpuUsage":20,"memory":{"total":100,"used":50},"storage":{"total":100,"used":10}}`}, nil
		}
		return sampleBroker{`{"cpu":6,"cpuUsage":60,"memory":{"total":300,"used":150},"storage":{"total":300,"used":90}}`}, nil
	}
	got := h.dashboardTelemetry(context.Background())
	usage := got["utilization"].(map[string]any)
	if got["telemetry_status"] != "live" || usage["cpu_percent"] != float64(50) || usage["memory_percent"] != float64(50) || usage["storage_percent"] != float64(25) {
		t.Fatalf("unexpected aggregate: %#v", got)
	}
	h.Connect = func(c config.NativeHostConfig) (nativeBroker, error) {
		if c.Address == "two" {
			return nil, errors.New("offline")
		}
		return sampleBroker{`{"cpu":2,"cpuUsage":0,"memory":{"total":100,"used":0},"storage":{"total":100,"used":0}}`}, nil
	}
	got = h.dashboardTelemetry(context.Background())
	if got["telemetry_status"] != "partial" || got["utilization"].(map[string]any)["cpu_percent"] != nil {
		t.Fatalf("partial samples presented as complete: %#v", got)
	}
}

func TestTelemetryRejectsMissingCPUAndImpossibleMemory(t *testing.T) {
	for _, raw := range []string{`{"cpu":2,"memory":{"total":100,"used":10},"storage":{"total":100,"used":10}}`, `{"cpu":2,"cpuUsage":0,"memory":{"total":100,"used":101},"storage":{"total":100,"used":10}}`} {
		var s hostMeasurement
		if err := json.Unmarshal([]byte(raw), &s); err != nil {
			t.Fatal(err)
		}
		if s.valid() {
			t.Fatalf("accepted invalid measurement: %s", raw)
		}
	}
}

func TestCombinedClusterHealth(t *testing.T) {
	for _, tc := range []struct{ host, ceph, want string }{
		{"Healthy", "HEALTH_OK", "Healthy"}, {"Unknown", "HEALTH_OK", "Unknown"},
		{"Healthy", "HEALTH_WARN", "Warning"}, {"Healthy", "HEALTH_ERR", "Degraded"},
		{"Degraded", "HEALTH_OK", "Degraded"}, {"Healthy", "", "Unknown"},
	} {
		if got := combinedClusterHealth(tc.host, tc.ceph); got != tc.want {
			t.Errorf("%v: %s", tc, got)
		}
	}
}

func TestLiveInventoryRequiresVerifiedFacts(t *testing.T) {
	h := NewNativeHandler(nil, map[string]config.NativeHostConfig{"kvm11": {Address: "192.0.2.11:22"}})
	h.Connect = func(config.NativeHostConfig) (nativeBroker, error) {
		return sampleBroker{`{"hostname":"kvm11.example","kernel":"6.0","os":"VM Alpha","cpu":16,"cpuModel":"test","physicalInterfaces":["ens192"],"memory":{"total":34359738368}}`}, nil
	}
	inventory, err := h.LiveInventory(context.Background(), "kvm11")
	if err != nil || inventory.CPU != 16 || inventory.Memory.Total != 34359738368 {
		t.Fatalf("unexpected verified inventory: %#v, %v", inventory, err)
	}
	h.Connect = func(config.NativeHostConfig) (nativeBroker, error) {
		return sampleBroker{`{"hostname":"kvm11.example","kernel":"","os":"VM Alpha","cpu":16,"memory":{"total":34359738368}}`}, nil
	}
	if _, err := h.LiveInventory(context.Background(), "kvm11"); err == nil {
		t.Fatal("accepted incomplete inventory")
	}
}

func TestStorageRegistrationUsesOnlyAllowlistedBrokerQuery(t *testing.T) {
	h := NewNativeHandler(nil, map[string]config.NativeHostConfig{"kvm11": {Address: "192.0.2.11:22"}})
	h.Connect = func(config.NativeHostConfig) (nativeBroker, error) {
		return sampleBroker{`{"version":1,"status":"verified"}`}, nil
	}
	raw, err := h.StorageRegistration(context.Background(), "kvm11")
	if err != nil || !json.Valid(raw) {
		t.Fatalf("unexpected registration result: %s, %v", raw, err)
	}
	if _, err := h.StorageRegistration(context.Background(), "missing"); err == nil {
		t.Fatal("accepted an unenrolled host")
	}
}
