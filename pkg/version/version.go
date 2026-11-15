package version

import "runtime/debug"

// Version is set at build time with -ldflags "-X github.com/nimbolus/terraform-backend/pkg/version.Version=v0.3.0".
var Version = ""

// Get returns the version of the binary. Without a version set at build time, it falls back to the module version,
// which `go install` sets.
func Get() string {
	if Version != "" {
		return Version
	}

	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}

	return "dev"
}
