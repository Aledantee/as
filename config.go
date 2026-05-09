package as

import (
	"context"
	"strings"

	"github.com/knadh/koanf/providers/env/v2"
	"github.com/knadh/koanf/v2"
	"go.aledante.io/ae"
)

// LoadEnvConfig loads configuration from environment variables into a struct of type T.
// It reads environment variables with the prefix defined in the context (via EnvPrefix),
// transforms keys by converting them to lowercase and replacing underscores with dots,
// and unmarshals them into the target struct.
//
// Environment variable keys are case-insensitive and support nesting via underscores.
// Values containing spaces are automatically split into string slices.
//
// Example:
//
//	With prefix "DEMO_API_":
//	DEMO_API_SERVER_PORT=9090	   → Config.Server.Port
//	DEMO_API_DATABASE_URL=...	   → Config.Database.URL
//	DEMO_API_TAGS="alpha beta"	  → Config.Tags = []string{"alpha", "beta"}
//
// The default struct tag used for unmarshaling is "config". Use WithConfigTag option
// to specify a different tag.
func LoadEnvConfig[T any](ctx context.Context, opts ...ConfigOption) (*T, error) {
	return loadEnvConfig[T](ctx, configOptions(opts...))
}

func loadEnvConfig[T any](ctx context.Context, opts ConfigOptions) (*T, error) {
	p := env.Provider(".", env.Opt{
		Prefix: EnvPrefix(ctx),
		TransformFunc: func(k, v string) (string, any) {
			k = strings.ReplaceAll(strings.ToLower(strings.TrimPrefix(k, EnvPrefix(ctx))), "_", ".")

			if strings.Contains(v, " ") {
				return k, strings.Split(v, " ")
			}

			return k, v
		},
	})

	k := koanf.New(".")
	if err := k.Load(p, nil); err != nil {
		return nil, ae.Wrap("failed to load environment variables", err)
	}

	o := new(T)
	_ = k.UnmarshalWithConf("", o, koanf.UnmarshalConf{
		Tag: opts.Tag,
	})

	return o, nil
}
