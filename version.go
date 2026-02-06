package as

import "context"

// versionKey is an unexported type used as the key for storing the version value in a context.
type versionKey struct{}

// withVersion returns a new context that carries the provided version string.
// If version is empty, it returns the original context unchanged.
func withVersion(ctx context.Context, version string) context.Context {
	if version == "" {
		return ctx
	}

	return context.WithValue(ctx, versionKey{}, version)
}

// Version extracts the version string from the context if present.
// If no version is set, it returns the empty string.
func Version(ctx context.Context) string {
	v, ok := ctx.Value(versionKey{}).(string)
	if !ok {
		return ""
	}

	return v
}
