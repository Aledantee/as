package as

import (
	"context"
	"os"
	"strings"
	"unicode"

	"github.com/caarlos0/env/v11"
	"golang.org/x/text/unicode/norm"
)

// envPrefixKey is an unexported type used as a key for storing the environment prefix in a context.
type envPrefixKey struct{}

// WithEnvPrefix returns a new context.Context derived from ctx that contains the given environment variable prefix.
func withEnvPrefix(ctx context.Context, prefix string) context.Context {
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

// NormalizeEnvKey normalizes a string for use as an environment variable key.
// It performs the following steps:
//   - Decomposes accented Unicode characters (e.g., "é" becomes "e" + acute); combining marks are removed.
//   - Converts all letters to uppercase.
//   - Replaces any non-alphanumeric character with a single underscore.
//   - Collapses consecutive non-alphanumeric characters into a single underscore.
//   - Trims leading and trailing underscores.
//
// The resulting keys are POSIX-safe, consisting only of [A-Z0-9_] and fully uppercase.
// Example: "my-Énv.key" → "MY_ENV_KEY"
func NormalizeEnvKey(name string) string {
	// NFD decomposes e.g. é into e + combining acute; we drop combining marks below.
	name = norm.NFD.String(name)

	var out []rune
	for _, r := range name {
		// skip combining marks (accents)
		if unicode.In(r, unicode.Mn) {
			continue
		}

		// Convert to upper case, or underscore if not alphanumeric
		if (r >= 'A' && r <= 'Z') ||
			(r >= 'a' && r <= 'z') ||
			(r >= '0' && r <= '9') {
			out = append(out, unicode.ToUpper(r))
		} else {
			if len(out) > 0 && out[len(out)-1] == '_' {
				continue
			}
			out = append(out, '_')
		}
	}

	// Trim any leading or trailing underscores
	return strings.Trim(string(out), "_")
}
