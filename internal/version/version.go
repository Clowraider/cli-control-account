package version

import "strings"

// Version represents the current version of the plugin.
// It is the single source of truth for plugin versioning.
// In release builds, this value can be injected/overridden via:
// -ldflags "-X control-account/internal/version.Version=x.y.z"
var Version = "0.6.4"

func init() {
	// Ensure version string never retains a leading 'v' so that
	// UI badges formatted as "v" + Version never produce double "vv".
	Version = strings.TrimPrefix(Version, "v")
}
