package handlers

import "testing"

func TestInjectGuestAgent(t *testing.T) {
	if got := injectGuestAgent("Rocky Linux", ""); got == "" || !isLinux("Rocky Linux") { t.Fatal("Linux defaults must include the guest agent") }
	if got := injectGuestAgent("Windows", ""); got != "" { t.Fatal("non-Linux defaults must remain unchanged") }
	if got := injectGuestAgent("Linux", "#cloud-config\nusers: []"); got != "#cloud-config\nusers: []" { t.Fatal("caller cloud-init must not be merged or overwritten") }
}
