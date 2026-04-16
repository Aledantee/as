package as_test

import (
	"context"
	"testing"

	"go.aledante.io/as"
)

func TestName_EmptyContextReturnsEmpty(t *testing.T) {
	t.Parallel()

	if got := as.Name(context.Background()); got != "" {
		t.Errorf("Name(background) = %q, want empty string", got)
	}
}

func TestNamespace_EmptyContextReturnsEmpty(t *testing.T) {
	t.Parallel()

	if got := as.Namespace(context.Background()); got != "" {
		t.Errorf("Namespace(background) = %q, want empty string", got)
	}
}

func TestVersion_EmptyContextReturnsEmpty(t *testing.T) {
	t.Parallel()

	if got := as.Version(context.Background()); got != "" {
		t.Errorf("Version(background) = %q, want empty string", got)
	}
}

// The "value present in context" cases are covered by run_test.go's
// TestRunC_ContextCarriesServiceIdentity, which exercises the documented
// pathway through RunC (the only public way to set these values).
