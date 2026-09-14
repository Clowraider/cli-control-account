package version_test

import (
	"testing"

	"control-account/internal/version"
)

func TestVersion_NotEmpty(t *testing.T) {
	if version.Version == "" {
		t.Fatal("expected version.Version to be non-empty")
	}
}

func TestVersion_CanBeMutatedByLdflags(t *testing.T) {
	orig := version.Version
	defer func() { version.Version = orig }()

	version.Version = "0.9.9-test"
	if version.Version != "0.9.9-test" {
		t.Fatalf("expected version.Version to be mutable for ldflags injection, got %q", version.Version)
	}
}

