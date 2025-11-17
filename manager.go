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

var errTimeout = fmt.Errorf("function did not complete within the given timeout")

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

	exitSignal chan int
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
		exitSignal:   make(chan int, 1),
	}
}

func (m *Manager) Add(name string, components Component) *Manager {
	m.components = append(m.components, namedComponent{name: name, Component: components})

	return m
}

func (m *Manager) Run() int {
	err := m.setupComponents()
	if errors.Is(err, errTimeout) {
		return int(syscall.SIGALRM)
	}
	if err != nil {
		return int(syscall.SIGABRT)
	}

	m.startComponents()

	signal := m.waitForSignal() // Wait for the exit signal

	err = m.closeComponents()
	if errors.Is(err, errTimeout) {
		return int(syscall.SIGALRM)
	}
	if err != nil {
		return int(syscall.SIGABRT)
	}

	return signal
}

func (m *Manager) setupComponents() error {
	for _, s := range m.components {
		setupable, ok := s.Component.(setupable)
		if ok {
			m.logInfo(fmt.Sprintf("Setting up component %q", s.name), slog.String("component_name", s.name))
			err := funcOrTimeout(setupable.Setup, m.setupTimeout)
			if errors.Is(err, errTimeout) {
				m.logError(fmt.Sprintf("Setup timed out for component %q", s.name), slog.String("component_name", s.name))
				return err
			}
			if err != nil {
				m.logError(fmt.Sprintf("Failure during setup for component %q: %v", s.name, err), slog.String("component_name", s.name))
				return err
			}
		}
	}
	return nil
}

func (m *Manager) startComponents() {
	for _, s := range m.components {
		startable, ok := s.Component.(startable)
		if ok {
			m.logInfo(fmt.Sprintf("Starting component %q", s.name), slog.String("component_name", s.name))
			go func() {
				defer func() {
					if r := recover(); r != nil {
						m.logError(fmt.Sprintf("Panic during start for component %q: %v", s.name, r), slog.String("component_name", s.name))
						m.exitSignal <- int(syscall.SIGABRT)
					}
				}()
				err := startable.Start() // Blocking for go routine
				if err != nil {
					m.logError(fmt.Sprintf("Failure during start for component %q: %v", s.name, err), slog.String("component_name", s.name))
					m.exitSignal <- int(syscall.SIGABRT)
				}
			}()
		}
	}
}

func (m *Manager) waitForSignal() int {
	go func() {
		select {
		case m.exitSignal <- m.lifetime():
		default:
			// Signal already sent, don't block
		}
	}()

	signal := <-m.exitSignal
	m.logInfo(fmt.Sprintf("Received signal: %d", signal), slog.Int("signal", signal))
	return signal
}

func (m *Manager) closeComponents() error {
	for _, s := range slices.Backward(m.components) {
		closable, ok := s.Component.(closable)
		if ok {
			m.logInfo(fmt.Sprintf("Closing component %q", s.name), slog.String("component_name", s.name))
			err := funcOrTimeout(closable.Close, m.closeTimeout)
			if errors.Is(err, errTimeout) {
				m.logError(fmt.Sprintf("Close timed out for component %q", s.name), slog.String("component_name", s.name))
				return err
			}
			if err != nil {
				m.logError(fmt.Sprintf("Failure during close for component %q: %v", s.name, err), slog.String("component_name", s.name))
				return err
			}
		}
	}

	return nil
}

func (m *Manager) logInfo(msg string, attrs ...any) {
	m.logger.Info("[UnixCycle] "+msg, attrs...)
}

func (m *Manager) logError(msg string, attrs ...any) {
	m.logger.Error("[UnixCycle] "+msg, attrs...)
}

// NOTE: goroutine may leak on timeout, but acceptable since timeout usually always leaves to a library shutdown
func funcOrTimeout(f func() error, timeout time.Duration) error {
	errs := make(chan error, 1)
	go func() {
		errs <- f()
	}()

	select {
	case err := <-errs:
		return err
	case <-time.After(timeout):
		return errTimeout
	}
}
