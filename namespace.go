package as

import "context"

// namespaceKey is an unexported type used as the key for storing the namespace value in a context.
type namespaceKey struct{}

// withNamespace returns a new context based on ctx that contains the provided namespace string,
// associated with a private key so it can be retrieved later. If the namespace value is empty, the
// original context is returned unchanged.
func withNamespace(ctx context.Context, domain string) context.Context {
	if domain == "" {
		return ctx
	}
	return context.WithValue(ctx, namespaceKey{}, domain)
}

// Namespace extracts the namespace string from the context if present, returning it. If no namespace has
// been set, it returns the empty string.
func Namespace(ctx context.Context) string {
	v, ok := ctx.Value(namespaceKey{}).(string)
	if !ok {
		return ""
	}
	return v
}
