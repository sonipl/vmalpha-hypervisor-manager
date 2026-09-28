package handlers

import "testing"

func TestTemplateDatastorePathIsStrict(t *testing.T) {
	for _, value := range []string{"templates/base.qcow2", "templates/rocky-9.qcow2"} {
		if _, err := templateDatastorePath(value); err != nil {
			t.Fatalf("rejected %q: %v", value, err)
		}
	}
	for _, value := range []string{"base.qcow2", "templates/sub/base.qcow2", "templates/base.raw", "templates/../secret.qcow2", "/templates/base.qcow2", "templates/.qcow2"} {
		if _, err := templateDatastorePath(value); err == nil {
			t.Fatalf("accepted %q", value)
		}
	}
}

func TestParseQCOWInfoRejectsBackingChain(t *testing.T) {
	if _, err := parseQCOWInfo([]byte(`{"format":"qcow2","virtual-size":1073741824}`)); err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{
		`{"format":"raw","virtual-size":1073741824}`,
		`{"format":"qcow2","virtual-size":0}`,
		`{"format":"qcow2","virtual-size":1073741824,"backing-filename":"parent.qcow2"}`,
	} {
		if _, err := parseQCOWInfo([]byte(raw)); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
}
