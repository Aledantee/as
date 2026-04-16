package as_test

import (
	"testing"

	"go.aledante.io/as"
)

func TestVCSVersion_DoesNotPanicAndReturnsString(t *testing.T) {
	t.Parallel()

	// Documented contract: returns the VCS revision as a string, or empty
	// string if unavailable. We cannot assert a specific value because the
	// test binary's build info depends on how it was built, but the call
	// itself must be safe and return a string.
	_ = as.VCSVersion()
}
