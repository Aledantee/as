package as

import (
	"context"
)

// Service defines the interface for an service implementation.
type Service interface {
	// Name returns the service name.
	Name() string
	// Namespace returns the service namespace.
	Namespace() string
	// Version returns the service version.
	Version() string

	// Init performs initialization logic for the service.
	Init(ctx context.Context) error

	// Run starts the main execution loop of the service.
	// This method must block until the service is stopped or returns an error.
	Run(ctx context.Context) error

	// Close cleans up any resources used by the service.
	Close(ctx context.Context) error
}
