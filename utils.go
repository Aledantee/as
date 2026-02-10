package as

import "runtime/debug"

// VCSVersion returns the current VCS (version control system) revision of the built binary,
// as determined by the "vcs.revision" setting available in the build info. If no revision
// is available, it returns the empty string.
//
// This is typically filled in during builds with module support using Go 1.18+.
func VCSVersion() string {
	bi, _ := debug.ReadBuildInfo()
	for _, setting := range bi.Settings {
		if setting.Key == "vcs.revision" && setting.Value != "" {
			return setting.Value
		}
	}
	return ""
}
