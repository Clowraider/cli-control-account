package version

// Version represents the current version of the plugin.
// In release builds, this value is injected via:
// -ldflags "-X control-account/internal/version.Version=x.y.z"
var Version = "0.6.0"
