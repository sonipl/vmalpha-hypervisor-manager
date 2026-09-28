package handlers

import (
	"context"
	"github.com/novasphere/novasphere/internal/config"
	"testing"
)

func TestNativeCephHealthValidation(t *testing.T) {
	for _, tc := range []struct{ raw, want string }{{`{"health":{"status":"HEALTH_OK"}}`, "HEALTH_OK"}, {`{"health":{"status":"HEALTH_WARN"}}`, "HEALTH_WARN"}, {`{}`, ""}, {`{"health":{"status":"pretend"}}`, ""}} {
		n := NewNativeHandler(nil, map[string]config.NativeHostConfig{"one": {Address: "one"}})
		n.Connect = func(config.NativeHostConfig) (nativeBroker, error) { return sampleBroker{tc.raw}, nil }
		h := &CephMetricsHandler{native: n}
		got, err := h.HealthStatus(context.Background())
		if got != tc.want || (err == nil) != (tc.want != "") {
			t.Fatalf("got %q %v", got, err)
		}
	}
	h := &CephMetricsHandler{}
	if _, err := h.HealthStatus(context.Background()); err == nil {
		t.Fatal("missing enrollment marked healthy")
	}
}
