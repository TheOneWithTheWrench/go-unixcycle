package unixcycle

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"syscall"
	"time"
)

var (
	errFuncTimeout = fmt.Errorf("function did not complete within the given timeout")
)

func defaultOptions() *managerOptions {
	return &managerOptions{
		logger:       slog.New(slog.NewTextHandler(os.Stdout, nil)),
		setupTimeout: 5 * time.Second,
		closeTimeout: 5 * time.Second,
		lifetime:     InterruptSignal,
	}
}

type Manager struct {
	components []namedComponent

	logger       *slog.Logger
	setupTimeout time.Duration
	closeTimeout time.Duration
	lifetime     TerminationSignal
}

func NewManager(options ...managerOption) *Manager {
	ops := defaultOptions()
	for _, o := range options {
		o(ops)
	}

	return &Manager{
		logger:       ops.logger,
		setupTimeout: ops.setupTimeout,
		closeTimeout: ops.closeTimeout,
		lifetime:     ops.lifetime,
	}
}

func (m *Manager) Add(name string, components Component) *Manager {
	m.components = append(m.components, namedComponent{name: name, Component: components})

	return m
}

func (m *Manager) Run() syscall.Signal {
	for _, s := range m.components {
		setupable, ok := s.Component.(setupable)
		if ok {
			m.logger.Info(fmt.Sprintf("[UnixCycle] Setting up component %q", s.name), slog.String("component_name", s.name))
			err := funcOrTimeout(setupable.Setup, m.setupTimeout)
			if errors.Is(err, errFuncTimeout) {
				m.logger.Error(fmt.Sprintf("[UnixCycle] Setup timed out for component %q", s.name), slog.String("component_name", s.name))
				return syscall.SIGALRM
			}
			if err != nil {
				m.logger.Error(fmt.Sprintf("[UnixCycle] Failure during setup for component %q: %v", s.name, err), slog.String("component_name", s.name))
				return syscall.SIGABRT
			}
		}
	}

	for _, s := range m.components {
		startable, ok := s.Component.(startable)
		if ok {
			m.logger.Info(fmt.Sprintf("[UnixCycle] Starting component %q", s.name), slog.String("component_name", s.name))
			go func() {
				err := startable.Start() // Blocking for go routine
				if err != nil {
					//TODO: We need to signal the manager somehow that a stop failed...
					m.logger.Error(fmt.Sprintf("[UnixCycle] Failure during start for component %q: %v", s.name, err), slog.String("component_name", s.name))
				}
			}()
		}
	}

	signal := m.lifetime() // Wait for the exit signal

	for _, s := range slices.Backward(m.components) {
		closable, ok := s.Component.(closable)
		if ok {
			m.logger.Info(fmt.Sprintf("[UnixCycle] Closing component %q", s.name), slog.String("component_name", s.name))
			err := funcOrTimeout(closable.Close, m.closeTimeout)
			if errors.Is(err, errFuncTimeout) {
				m.logger.Error(fmt.Sprintf("[UnixCycle] Close timed out for component %q", s.name), slog.String("component_name", s.name))
				return syscall.SIGALRM
			}
			if err != nil {
				m.logger.Error(fmt.Sprintf("[UnixCycle] Failure during close for component %q: %v", s.name, err), slog.String("component_name", s.name))
				return syscall.SIGABRT
			}
		}
	}

	return signal
}

func funcOrTimeout(f func() error, timeout time.Duration) error {
	errs := make(chan error, 1)
	go func() {
		errs <- f()
	}()

	select {
	case err := <-errs:
		return err
	case <-time.After(timeout):
		return errFuncTimeout
	}
}
