package unixcycle

import (
	"errors"
	"fmt"
	"syscall"
	"time"
)

var (
	errFuncTimeout = fmt.Errorf("function did not complete within the given timeout")
)

func defaultOptions() *managerOptions {
	return &managerOptions{
		setupTimeout: 5 * time.Second,
		closeTimeout: 5 * time.Second,
		lifetime:     InterruptSignal,
	}
}

type Manager struct {
	components []Component

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
		setupTimeout: ops.setupTimeout,
		closeTimeout: ops.closeTimeout,
		lifetime:     ops.lifetime,
	}
}

func (m *Manager) Add(components ...Component) *Manager {
	m.components = append(m.components, components...)

	return m
}

// TODO: Every component should be started SYNCHRONOUSLY in their own goroutine
// Since we will be pausing this thread with the TerminationSignal function
func (m *Manager) Run() syscall.Signal {
	for _, s := range m.components { //TODO: We need some sort of timeout here... So we don't block forever
		setupable, ok := s.(setupable)
		if ok {
			err := funcOrTimeout(setupable.Setup, m.setupTimeout)
			if errors.Is(err, errFuncTimeout) {
				fmt.Printf("Setup timed out for component: %T\n", s)
				return syscall.SIGALRM
			}
			if err != nil {
				fmt.Printf("Failure during setup for component %T: %v\n", s, err)
				return syscall.SIGABRT
			}
		}
	}

	for _, s := range m.components {
		startable, ok := s.(startable)
		if ok {
			go func() { //TODO: We need to be able to stop this... Right? Or is that the Closer's job?
				err := startable.Start() // Blocking for go routine
				if err != nil {
					fmt.Printf("Failure during start for component %T: %v\n", s, err)
				}
			}()
		}
	}

	signal := m.lifetime() // Wait for the exit signal

	for _, s := range m.components { //TODO: Same here, we need a timeout here
		closable, ok := s.(closable)
		if ok {
			err := funcOrTimeout(closable.Close, m.closeTimeout)
			if errors.Is(err, errFuncTimeout) {
				fmt.Printf("Close timed out for component: %T\n", s)
				return syscall.SIGALRM
			}
			if err != nil {
				fmt.Printf("Failure during close for component %T: %v\n", s, err)
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
