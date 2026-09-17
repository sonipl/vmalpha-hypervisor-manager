package config

import (
	"strings"
	"testing"
)

func TestSigningKeyRequired(t *testing.T) {
	for _, key := range []string{"", "short", strings.Repeat("x", 31)} {
		if (Config{Port: 8080, Auth: AuthConfig{JWTSecret: key}}).Validate() == nil {
			t.Fatal("weak or missing signing key accepted")
		}
	}
	if err := (Config{Port: 8080, Auth: AuthConfig{JWTSecret: strings.Repeat("x", 32)}}).Validate(); err != nil {
		t.Fatal(err)
	}
}
