package as

// ConfigOptions holds configuration options for loading environment variables.
type ConfigOptions struct {
	// Tag specifies the struct tag name to use when unmarshaling configuration.
	// Default is "config".
	Tag string
}

// ConfigOption is a function that modifies ConfigOptions.
type ConfigOption func(*ConfigOptions)

func configOptions(opts ...ConfigOption) ConfigOptions {
	var options ConfigOptions
	for _, opt := range opts {
		opt(&options)
	}

	// sanity guard against an empty tag
	if options.Tag == "" {
		options.Tag = "config"
	}

	return options
}

// WithConfigTag returns a ConfigOption that sets the struct tag name for unmarshaling.
// If an empty string is provided, the tag will not be modified from its default or previously set value.
func WithConfigTag(tag string) ConfigOption {
	return func(o *ConfigOptions) {
		if tag != "" {
			o.Tag = tag
		}
	}
}
