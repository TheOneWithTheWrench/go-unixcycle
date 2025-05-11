package unixcycle_test

import (
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/theonewiththewrench/unixcycle"
)

func TestManager(t *testing.T) {
	var (
		manualSignal = func(signalChan chan syscall.Signal) unixcycle.TerminationSignal {
			return func() syscall.Signal {
				return <-signalChan
			}
		}
		newManager = func() (*unixcycle.Manager, func(syscall.Signal)) {
			shutdownChan := make(chan syscall.Signal, 1)
			m := unixcycle.NewManager(
				unixcycle.WithLifetime(manualSignal(shutdownChan)),
				unixcycle.WithSetupTimeout(100*time.Millisecond),
				unixcycle.WithCloseTimeout(100*time.Millisecond),
			)
			return m, func(signal syscall.Signal) { shutdownChan <- signal }
		}
	)

	t.Run("should call setup on setupable component", func(t *testing.T) {
		var (
			m, shutdown = newManager()
			calledCount = 0
			setupable   = func() error {
				defer shutdown(syscall.Signal(0))
				calledCount++
				return nil
			}
			sut = m.Add("setup func", unixcycle.Setup(setupable))
		)

		got := sut.Run()

		assert.Equal(t, calledCount, 1)
		assert.Equal(t, syscall.Signal(0), got)
	})

	t.Run("should call start on startable component", func(t *testing.T) {
		var (
			m, shutdown = newManager()
			calledCount = 0
			startable   = func() error {
				defer shutdown(syscall.Signal(0))
				calledCount++
				return nil
			}
			sut = m.Add("startable func", unixcycle.Starter(startable))
		)

		got := sut.Run()

		assert.Equal(t, calledCount, 1)
		assert.Equal(t, syscall.Signal(0), got)
	})

	t.Run("should call close on closable component", func(t *testing.T) {
		var (
			m, shutdown = newManager()
			calledCount = 0
			closable    = func() error {
				calledCount++
				return nil
			}
			sut = m.Add("closeable func", unixcycle.Closer(closable))
		)

		shutdown(syscall.Signal(0)) // We can't shutdown from the closer func. Since it's called AFTER the signal is received
		got := sut.Run()

		assert.Equal(t, calledCount, 1)
		assert.Equal(t, syscall.Signal(0), got)
	})

	t.Run("should call setup, start and close functions on component if they are present", func(t *testing.T) {
		var (
			m, shutdown = newManager()
			testComp    = &testComponent{
				setupFunc: func() error { return nil },
				startFunc: func() error { shutdown(syscall.Signal(0)); return nil },
				closeFunc: func() error { return nil },
			}
			sut = m.Add("make func", unixcycle.Make[testComponent](testComp))
		)

		got := sut.Run()

		assert.Equal(t, 1, testComp.setupCalledCount)
		assert.Equal(t, 1, testComp.startCalledCount)
		assert.Equal(t, 1, testComp.closeCalledCount)
		assert.Equal(t, syscall.Signal(0), got)
	})

	// This test plays a bit with races in the setup of the manager.
	// We artificially slow down the setup function to make sure that the manager times out
	// and then asserts that the setup function was not called.
	t.Run("should receive SIGALRM if setup times out", func(t *testing.T) {
		var (
			m, _        = newManager()
			calledCount = atomic.Uint32{} // Have to use atomic here due to playing with race conditions
			slowSetup   = func() error {
				time.Sleep(200 * time.Millisecond) // Slower than the 100ms timeout
				calledCount.Add(1)                 // This should not be called due to the timeout
				return nil
			}
			sut = m.Add("slow func", unixcycle.Setup(slowSetup))
		)

		got := sut.Run()

		assert.Equal(t, uint32(0), calledCount.Load())
		assert.Equal(t, syscall.SIGALRM, got)
	})
}

type testComponent struct {
	setupCalledCount int
	startCalledCount int
	closeCalledCount int

	setupFunc func() error
	startFunc func() error
	closeFunc func() error
}

func (c *testComponent) Setup() error {
	c.setupCalledCount++
	return c.setupFunc()
}

func (c *testComponent) Start() error {
	c.startCalledCount++
	return c.startFunc()
}

func (c *testComponent) Close() error {
	c.closeCalledCount++
	return c.closeFunc()
}
