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
