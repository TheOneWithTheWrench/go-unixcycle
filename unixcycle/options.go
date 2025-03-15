package unixcycle

import (
	"io"
	"log/slog"
	"time"
)

type managerOption func(*managerOptions)

type managerOptions struct {
	logger       *slog.Logger
	setupTimeout time.Duration
	closeTimeout time.Duration
	lifetime     TerminationSignal
}

func WithLifetime(lifetime TerminationSignal) managerOption {
	return func(o *managerOptions) {
		o.lifetime = lifetime
	}
}

// WithSetupTimeout sets the timeout that EACH component has to setup
// before the manager will consider the setup failed
// Default is 5 seconds
func WithSetupTimeout(timeout time.Duration) managerOption {
	return func(o *managerOptions) {
		o.setupTimeout = timeout
	}
}

// WithCloseTimeout sets the timeout that EACH component has to close
// before the manager will consider the close failed
// Default is 5 seconds
func WithCloseTimeout(timeout time.Duration) managerOption {
	return func(o *managerOptions) {
		o.closeTimeout = timeout
	}
}

// WithLoggingHandler sets the logger for the manager
// If handler is nil, the manager will log nothing
// Default is a text logging handler that writes to os.Stdout
func WithLoggingHandler(handler slog.Handler) managerOption {
	return func(o *managerOptions) {
		if handler == nil {
			o.logger = slog.New(slog.NewTextHandler(io.Discard, nil))
			return
		}

		o.logger = slog.New(handler)
	}
}
