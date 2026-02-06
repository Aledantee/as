package as

import (
	"runtime/debug"
	"time"

	"github.com/caarlos0/env/v11"
)

// Options defines the configuration parameters for the lifecycle and supervision
// of a service instance. These control restart policies, shutdown handling,
// and debug logging. All time-based fields are expressed as time.Duration.
//
// After applying Option funcs, options are merged with environment variables:
// the effective env prefix (see EnvPrefix) is normalized and used with
// env.ParseWithOptions, so any option may be overridden by a matching env var
// (e.g. PREFIX_RESTART_ON_ERROR, PREFIX_GRACE_PERIOD).
//
// The zero value of Options is not valid: use DefaultOptions or applyOptions to obtain sensible defaults.
type Options struct {
	// RestartOnError enables automatic service restarts upon encountering an error.
	// The number of allowed restarts is governed by GraceCount and GracePeriod, whichever is reached first.
	RestartOnError bool `env:"RESTART_ON_ERROR"`
	// RestartOnErrorDelay specifies the delay between consecutive restarts due to errors.
	RestartOnErrorDelay time.Duration `env:"RESTART_ON_ERROR_DELAY"`
	// RestartOnPanic enables automatic restarts when the service panics.
	RestartOnPanic bool `env:"RESTART_ON_PANIC"`
	// RestartOnPanicDelay is the delay between restarts caused by a panic.
	// If unset (zero), RestartOnErrorDelay is used.
	RestartOnPanicDelay time.Duration `env:"RESTART_ON_PANIC_DELAY"`
	// RecoverPanic enables automatic recovery from panics in the service main loop.
	// If true, panics will be converted and handled as normal service errors.
	RecoverPanic bool `env:"RECOVER_PANIC"`
	// GracePeriod is the maximum duration after the initial start during which retries are allowed.
	// If set to zero, there is no time limit.
	GracePeriod time.Duration `env:"GRACE_PERIOD"`
	// GraceCount is the total number of allowed restarts after the first start.
	// If set to zero, there is no limit.
	GraceCount int `env:"GRACE_COUNT"`
	// ShutdownTimeout is the maximum duration to wait when shutting down the service gracefully.
	// If the service shutdown takes longer than this, it will be forcefully terminated. Any restart config
	// will be ignored.
	ShutdownTimeout time.Duration `env:"SHUTDOWN_TIMEOUT"`
	// LogDebug enables verbose debug logging for this service.
	// Defaults to true when the source tree has local modifications.
	// Implicitly disables JSON logging when enabled.
	LogDebug bool `env:"LOG_DEBUG"`
	// LogJson enables JSON-formatted logging output.
	LogJson bool `env:"LOG_JSON"`
	// LogColors enables colorized logging output. Ignored if LogJson is true.
	LogColors bool `env:"LOG_COLORS"`
	// LogAutoColors enables colorized logging output if stdout is a terminal.
	LogAutoColors bool `env:"LOG_COLORS_AUTO"`
	// EnvPrefix is the prefix used when loading Options from the environment.
	// If empty, the prefix is derived from the service namespace and name:
	// "<namespace>_<name>_" when namespace is set, otherwise "<name>_".
	// The final prefix is normalized with NormalizeEnvKey before use.
	// Option fields are then filled from env vars like PREFIX_RESTART_ON_ERROR, PREFIX_GRACE_PERIOD, etc.
	EnvPrefix string
}

// DefaultOptions returns an Options struct pre-populated with recommended default values
// for robust service supervision. Callers may further modify the returned struct.
func DefaultOptions() Options {
	bi, ok := debug.ReadBuildInfo()
	logDebug := false
	if ok {
		for _, s := range bi.Settings {
			if s.Key == "vcs.modified" {
				logDebug = true
			}
		}
	}

	return Options{
		RestartOnError:      true,
		RestartOnErrorDelay: 10 * time.Second,
		RestartOnPanic:      true,
		RecoverPanic:        true,
		GracePeriod:         1 * time.Minute,
		GraceCount:          3,
		ShutdownTimeout:     30 * time.Second,
		LogDebug:            logDebug,
		LogAutoColors:       true,
		LogJson:             true,
	}
}

// Option is a function which applies a configuration change to the Options struct.
// Use higher-level constructors or WithXyz style helpers to supply these.
type Option func(*Options)

func WithRestartOnError(v bool) Option {
	return func(o *Options) { o.RestartOnError = v }
}

func WithRestartOnErrorDelay(v time.Duration) Option {
	return func(o *Options) { o.RestartOnErrorDelay = v }
}

func WithRestartOnPanic(v bool) Option {
	return func(o *Options) { o.RestartOnPanic = v }
}

func WithRestartOnPanicDelay(v time.Duration) Option {
	return func(o *Options) { o.RestartOnPanicDelay = v }
}

func WithRecoverPanic(v bool) Option {
	return func(o *Options) { o.RecoverPanic = v }
}

func WithGracePeriod(v time.Duration) Option {
	return func(o *Options) { o.GracePeriod = v }
}

func WithGraceCount(v int) Option {
	return func(o *Options) { o.GraceCount = v }
}

func WithShutdownTimeout(v time.Duration) Option {
	return func(o *Options) { o.ShutdownTimeout = v }
}

func WithLogDebug(v bool) Option {
	return func(o *Options) { o.LogDebug = v }
}

func WithLogJson(v bool) Option {
	return func(o *Options) { o.LogJson = v }
}

func WithLogColors(v bool) Option {
	return func(o *Options) { o.LogColors = v }
}

func WithLogAutoColors(v bool) Option {
	return func(o *Options) { o.LogAutoColors = v }
}

// applyOptions builds Options by applying the given Option funcs to DefaultOptions(),
// then overlaying environment variables. The env prefix is: EnvPrefix if non-empty;
// otherwise "<namespace>_<name>_" (namespace omitted if empty). The prefix is
// normalized with NormalizeEnvKey and passed to env.ParseWithOptions so that
// Options fields (e.g. RESTART_ON_ERROR, GRACE_PERIOD) can be set via prefixed env vars.
func applyOptions(name, namespace string, opts []Option) Options {
	o := DefaultOptions()
	for _, opt := range opts {
		opt(&o)
	}

	envPrefix := o.EnvPrefix
	if envPrefix == "" {
		if namespace != "" {
			envPrefix = namespace + "_"
		}
		envPrefix = envPrefix + name + "_"
	}

	_ = env.ParseWithOptions(&o, env.Options{
		Prefix: NormalizeEnvKey(envPrefix),
	})

	return o
}
