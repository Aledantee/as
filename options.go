package as

import (
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
	// LogColors enables colorized logging output. Does nothing when using JSON logging.
	LogColors bool `env:"LOG_COLORS"`
	// LogAutoColors enables colorized logging output if stdout is a terminal.
	LogAutoColors bool `env:"LOG_COLORS_AUTO"`
	// EnvPrefix is the prefix used when loading Options from the environment.
	// If empty, the prefix is derived from the service namespace and name:
	// "<namespace>_<name>_" when namespace is set, otherwise "<name>_".
	// The final prefix is normalized with NormalizeEnvKey before use.
	// Option fields are then filled from env vars like PREFIX_RESTART_ON_ERROR, PREFIX_GRACE_PERIOD, etc.
	EnvPrefix string
	// DisableEnvPrefix, when set to true, prevents any environment variable prefix
	// from being applied when loading option values from the environment, regardless of EnvPrefix.
	// This can be used to disable all prefixing behavior for built-in options,
	// ensuring that option fields are matched exactly to their environment variable names
	// as defined by the `env` struct tags.
	// As with all env options, this will also impact the EnvPrefix behavior for the service context.
	DisableEnvPrefix bool
}

// DefaultOptions returns an Options struct pre-populated with recommended default values
// for robust service supervision. Callers may further modify the returned struct.
func DefaultOptions() Options {
	return Options{
		RestartOnError:      true,
		RestartOnErrorDelay: 10 * time.Second,
		RestartOnPanic:      true,
		RecoverPanic:        true,
		GracePeriod:         1 * time.Minute,
		GraceCount:          3,
		ShutdownTimeout:     30 * time.Second,
		LogDebug:            false,
		LogColors:           false,
		LogAutoColors:       true,
		LogJson:             true,
		EnvPrefix:           "",
		DisableEnvPrefix:    false,
	}
}

// Option is a function which applies a configuration change to the Options struct.
// Use higher-level constructors or WithXyz style helpers to supply these.
type Option func(*Options)

// WithRestartOnError sets the RestartOnError field, enabling or disabling automatic service restarts on error.
func WithRestartOnError(v bool) Option {
	return func(o *Options) { o.RestartOnError = v }
}

// WithRestartOnErrorDelay sets the delay between consecutive restarts due to errors.
func WithRestartOnErrorDelay(v time.Duration) Option {
	return func(o *Options) { o.RestartOnErrorDelay = v }
}

// WithRestartOnPanic sets the RestartOnPanic field, enabling or disabling restarts when the service panics.
func WithRestartOnPanic(v bool) Option {
	return func(o *Options) { o.RestartOnPanic = v }
}

// WithRestartOnPanicDelay sets the delay between restarts triggered by a panic.
func WithRestartOnPanicDelay(v time.Duration) Option {
	return func(o *Options) { o.RestartOnPanicDelay = v }
}

// WithRecoverPanic sets the RecoverPanic field, enabling or disabling panic recovery.
func WithRecoverPanic(v bool) Option {
	return func(o *Options) { o.RecoverPanic = v }
}

// WithGracePeriod sets the maximum duration after the initial start in which restarts are allowed.
func WithGracePeriod(v time.Duration) Option {
	return func(o *Options) { o.GracePeriod = v }
}

// WithGraceCount sets the maximum number of allowed restarts after the initial start.
func WithGraceCount(v int) Option {
	return func(o *Options) { o.GraceCount = v }
}

// WithShutdownTimeout sets the maximum time to wait for graceful service shutdown.
func WithShutdownTimeout(v time.Duration) Option {
	return func(o *Options) { o.ShutdownTimeout = v }
}

// WithLogDebug sets the LogDebug field, enabling or disabling verbose debug logging.
func WithLogDebug(v bool) Option {
	return func(o *Options) { o.LogDebug = v }
}

// WithLogJson sets the LogJson field, enabling or disabling JSON-formatted logging output.
func WithLogJson(v bool) Option {
	return func(o *Options) { o.LogJson = v }
}

// WithLogColors sets the LogColors field, enabling or disabling colorized logging output.
func WithLogColors(v bool) Option {
	return func(o *Options) { o.LogColors = v }
}

// WithLogAutoColors sets the LogAutoColors field, enabling or disabling automatic colorization if stdout is a terminal.
func WithLogAutoColors(v bool) Option {
	return func(o *Options) { o.LogAutoColors = v }
}

// WithDisableEnvPrefix sets the DisableEnvPrefix field, preventing any environment variable prefix from being applied.
func WithDisableEnvPrefix(v bool) Option {
	return func(o *Options) { o.DisableEnvPrefix = v }
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

	envPrefix := ""
	if !o.DisableEnvPrefix {
		envPrefix = o.EnvPrefix
		if envPrefix == "" {
			if namespace != "" {
				envPrefix = namespace + "_"
			}
			envPrefix = envPrefix + name + "_"
		}
	}

	if envPrefix != "" {
		o.EnvPrefix = NormalizeEnvKey(envPrefix) + "_"
	}

	_ = env.ParseWithOptions(&o, env.Options{
		Prefix: o.EnvPrefix,
	})

	return o
}
