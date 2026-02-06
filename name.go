package as

import "context"

// nameKey is an unexported type used as the key for storing the name value in a context.
type nameKey struct{}

// withName returns a new context based on ctx that contains the provided name string,
// associated with a private key so it can be retrieved later. If name is empty, the
// original context is returned unchanged.
func withName(ctx context.Context, name string) context.Context {
	if name == "" {
		return ctx
	}

	return context.WithValue(ctx, nameKey{}, name)
}

// Name extracts the name string from the context if present, returning it. If no name has
// been set, it returns the empty string.
func Name(ctx context.Context) string {
	v, ok := ctx.Value(nameKey{}).(string)
	if !ok {
		return ""
	}

	return v
}
