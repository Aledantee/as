package as

import (
	"runtime/debug"
	"time"
)

// Options defines the configuration parameters for the lifecycle and supervision
// of a service instance. These control restart policies, shutdown handling,
// and debug logging. All time-based fields are expressed as time.Duration.
//
// The zero value of Options is not valid: use DefaultOptions or applyOptions to obtain sensible defaults.
type Options struct {
	// RestartOnError enables automatic service restarts upon encountering an error.
	// The number of allowed restarts is governed by GraceCount and GracePeriod, whichever is reached first.
	RestartOnError bool
	// RestartOnErrorDelay specifies the delay between consecutive restarts due to errors.
	RestartOnErrorDelay time.Duration
	// RestartOnPanic enables automatic restarts when the service panics.
	RestartOnPanic bool
	// RestartOnPanicDelay is the delay between restarts caused by a panic.
	// If unset (zero), RestartOnErrorDelay is used.
	RestartOnPanicDelay time.Duration
	// RecoverPanic enables automatic recovery from panics in the service main loop.
	// If true, panics will be converted and handled as normal service errors.
	RecoverPanic bool
	// GracePeriod is the maximum duration after the initial start during which retries are allowed.
	// If set to zero, there is no time limit.
	GracePeriod time.Duration
	// GraceCount is the total number of allowed restarts after the first start.
	// If set to zero, there is no limit.
	GraceCount int
	// ShutdownTimeout is the maximum duration to wait when shutting down the service gracefully.
	// If the service shutdown takes longer than this, it will be forcefully terminated. Any restart config
	// will be ignored.
	ShutdownTimeout time.Duration
	// LogDebug enables verbose debug logging for this service.
	// Defaults to true when the source tree has local modifications.
	// Implicitly disables JSON logging when enabled.
	LogDebug bool
	// LogJson enables JSON-formatted logging output.
	LogJson bool
	// LogColors enables colorized logging output. Ignored if LogJson is true.
	LogColors bool
	// LogAutoColors enables colorized logging output if stdout is a terminal.
	LogAutoColors bool
	// EnvPrefix is the prefix to use for environment variables.
	// If empty, defaults <SERVICE>_ where service is the upper case name of the service name.
	// If Namespace is set, the prefix will default to <NAMESPACE>_<SERVICE>_
	// If the service name contains non-alphanumeric characters, they will be replaced with underscores.
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

// applyOptions applies a sequence of Option functions to a new Options struct initialized
// with DefaultOptions, yielding a complete Options configuration for service use.
func applyOptions(opts []Option) Options {
	o := DefaultOptions()
	for _, opt := range opts {
		opt(&o)
	}

	return o
}
