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
