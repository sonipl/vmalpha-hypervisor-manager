package handlers

import (
	"testing"

	"github.com/novasphere/novasphere/internal/models"
)

func TestRecoveredNativeStatus(t *testing.T) {
	for status, want := range map[models.HostStatus]models.HostStatus{
		models.HostStatusOffline:     models.HostStatusReady,
		models.HostStatusError:       models.HostStatusReady,
		models.HostStatusReady:       models.HostStatusReady,
		models.HostStatusMaintenance: models.HostStatusMaintenance,
		models.HostStatusDraining:    models.HostStatusDraining,
	} {
		if got := recoveredNativeStatus(status); got != want {
			t.Fatalf("%q recovered as %q, want %q", status, got, want)
		}
	}
}
