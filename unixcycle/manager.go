package unixcycle

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
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
	components []Component

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

func (m *Manager) Add(components ...Component) *Manager {
	m.components = append(m.components, components...)

	return m
}

func (m *Manager) Run() syscall.Signal {
	for _, s := range m.components {
		setupable, ok := s.(setupable)
		if ok {
			m.logger.Info(fmt.Sprintf("[UnixCycle] Setting up component of type %T", s), slog.String("component_type", fmt.Sprintf("%T", s)))
			err := funcOrTimeout(setupable.Setup, m.setupTimeout)
			if errors.Is(err, errFuncTimeout) {
				m.logger.Error(fmt.Sprintf("[UnixCycle] Setup timed out for component of type %T", s), slog.String("component_type", fmt.Sprintf("%T", s)))
				return syscall.SIGALRM
			}
			if err != nil {
				m.logger.Error(fmt.Sprintf("[UnixCycle] Failure during setup for component of type %T: %v", s, err), slog.String("component_type", fmt.Sprintf("%T", s)))
				return syscall.SIGABRT
			}
		}
	}

	for _, s := range m.components {
		startable, ok := s.(startable)
		if ok {
			m.logger.Info(fmt.Sprintf("[UnixCycle] Starting component of type %T", s), slog.String("component_type", fmt.Sprintf("%T", s)))
			go func() {
				err := startable.Start() // Blocking for go routine
				if err != nil {
					//TODO: We need to signal the manager somehow that a stop failed...
					m.logger.Error(fmt.Sprintf("[UnixCycle] Failure during start for component of type %T: %v", s, err), slog.String("component_type", fmt.Sprintf("%T", s)))
				}
			}()
		}
	}

	signal := m.lifetime() // Wait for the exit signal

	for _, s := range m.components {
		closable, ok := s.(closable)
		if ok {
			m.logger.Info(fmt.Sprintf("[UnixCycle] Closing component of type %T", s), slog.String("component_type", fmt.Sprintf("%T", s)))
			err := funcOrTimeout(closable.Close, m.closeTimeout)
			if errors.Is(err, errFuncTimeout) {
				m.logger.Error(fmt.Sprintf("[UnixCycle] Close timed out for component of type %T", s), slog.String("component_type", fmt.Sprintf("%T", s)))
				return syscall.SIGALRM
			}
			if err != nil {
				m.logger.Error(fmt.Sprintf("[UnixCycle] Failure during close for component of type %T: %v", s, err), slog.String("component_type", fmt.Sprintf("%T", s)))
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
