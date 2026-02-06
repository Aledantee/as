package as

import (
	"context"
	"os"

	"github.com/caarlos0/env/v11"
)

// envPrefixKey is an unexported type used as a key for storing the environment prefix in a context.
type envPrefixKey struct{}

// WithEnvPrefix returns a new context.Context derived from ctx that contains the given environment variable prefix.
func WithEnvPrefix(ctx context.Context, prefix string) context.Context {
	return context.WithValue(ctx, envPrefixKey{}, prefix)
}

// EnvPrefix retrieves the environment variable prefix from the context, if set via WithEnvPrefix.
// If no prefix is present, it returns an empty string.
func EnvPrefix(ctx context.Context) string {
	v, ok := ctx.Value(envPrefixKey{}).(string)
	if !ok {
		return ""
	}
	return v
}

// GetEnv retrieves the value of the environment variable named by the key, with any prefix set in the context applied.
// The key is normalized using NormalizeEnvKey before being used to retrieve the value from the environment.
func GetEnv(ctx context.Context, key string) string {
	return os.Getenv(NormalizeEnvKey(EnvPrefix(ctx) + key))
}

// LookupEnv retrieves the value of the environment variable named by the key, with any prefix in the context applied.
// It returns the value (after normalization) and a boolean indicating whether the variable was present.
// The key is normalized using NormalizeEnvKey before being used to retrieve the value from the environment.
func LookupEnv(ctx context.Context, key string) (string, bool) {
	v, ok := os.LookupEnv(EnvPrefix(ctx) + key)
	return NormalizeEnvKey(v), ok
}

// LoadEnv parses environment variables into a struct of type T, applying any prefix set in the context.
// It relies on the github.com/caarlos0/env/v11 library for parsing.
// Note: This does **NOT** normalize the environment variable keys using NormalizeEnvKey. The env keys from the tags are used as is.
func LoadEnv[T any](ctx context.Context) (T, error) {
	return env.ParseAsWithOptions[T](env.Options{
		Prefix: EnvPrefix(ctx),
	})
}

// normalizeEnvName processes the environment variable value before returning it from GetEnv or LookupEnv.
// This function currently returns the value unchanged, but can be extended for normalization logic.
func NormalizeEnvKey(name string) string {
	return name
}
