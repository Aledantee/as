package as

import "runtime/debug"

// VCSVersion returns the current VCS (version control system) revision of the built binary,
// as determined by the "vcs.revision" setting available in the build info. If no revision
// is available, it returns the empty string.
//
// This is typically filled in during builds with module support using Go 1.18+.
func VCSVersion() string {
	bi, _ := debug.ReadBuildInfo()
	if bi == nil {
		return ""
	}
	for _, setting := range bi.Settings {
		if setting.Key == "vcs.revision" && setting.Value != "" {
			return setting.Value
		}
	}
	return ""
}

// vcsModified reports whether the VCS working tree had local modifications
// when this binary was built. Returns false when build info is unavailable
// (e.g. binaries built without module support).
func vcsModified() bool {
	bi, _ := debug.ReadBuildInfo()
	if bi == nil {
		return false
	}
	for _, setting := range bi.Settings {
		if setting.Key == "vcs.modified" {
			return setting.Value == "true"
		}
	}
	return false
}
