package unixcycle_test

import (
	"context"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/theonewiththewrench/unixcycle"
)

func TestTesting(t *testing.T) {
	t.Parallel()

	const (
		retryDelay  = 100 * time.Millisecond
		timeout     = 1*time.Second + (retryDelay / 2) // Give us a bit of leeway... Ain't pretty
		maxAttempts = int(timeout / retryDelay)
	)

	t.Run("RetryingProber", func(t *testing.T) {
		t.Parallel()

		t.Run("should return timeout error if prober never succeeds", func(t *testing.T) {
			t.Parallel()
			// Arrange
			var (
				prober = &simpleMockProber{successAfterCalls: maxAttempts + 1}
				sut    = unixcycle.RetryingProber(retryDelay, timeout, prober.Probe)
			)

			// Act
			err := sut(context.Background())

			// Assert
			require.Error(t, err)
			assert.ErrorContains(t, err, "prober timed out")
			assert.Equal(t, maxAttempts, int(prober.calls.Load()))
		})

		t.Run("should return error if context is cancelled", func(t *testing.T) {
			t.Parallel()
			// Arrange
			var (
				prober = &simpleMockProber{successAfterCalls: maxAttempts + 1}
				sut    = unixcycle.RetryingProber(retryDelay, timeout, prober.Probe)
				ctx    = context.Background()
			)

			// Act
			ctx, cancel := context.WithCancel(ctx)
			go func() { // Cancel the context after half the timeout
				time.Sleep(timeout / 2)
				cancel()
			}()
			err := sut(ctx)

			// Assert
			require.Error(t, err)
			assert.ErrorContains(t, err, "retrying prober failed")
			assert.Equal(t, maxAttempts/2, int(prober.calls.Load()))
		})

		t.Run("should return nil if prober succeeds", func(t *testing.T) {
			t.Parallel()
			// Arrange
			var (
				successAfterCalls = maxAttempts - 1
				prober            = &simpleMockProber{successAfterCalls: successAfterCalls}
				sut               = unixcycle.RetryingProber(retryDelay, timeout, prober.Probe)
			)

			// Act
			err := sut(context.Background())

			// Assert
			require.NoError(t, err)
			assert.Equal(t, successAfterCalls, int(prober.calls.Load()))
		})
	})

	t.Run("ParallelProber", func(t *testing.T) {
		t.Parallel()

		t.Run("should return error if any prober never succeeds", func(t *testing.T) {
			t.Parallel()
			// Arrange
			var (
				prober1 = &simpleMockProber{successAfterCalls: 1}
				prober2 = &simpleMockProber{successAfterCalls: maxAttempts + 1} // This one will never succeed
				sut     = unixcycle.ParallelProber(prober1.Probe, prober2.Probe)
			)

			// Act
			err := sut(context.Background())

			// Assert
			require.Error(t, err)
			assert.ErrorContains(t, err, "parallel prober errored")
			assert.True(t, prober1.calls.Load() > 0)
			assert.True(t, prober2.calls.Load() > 0)
		})

		t.Run("should return timeout if a prober takes too long", func(t *testing.T) {
			t.Parallel()
			// Arrange
			var (
				ctx, cancel = context.WithTimeout(context.Background(), timeout)
				prober1     = &simpleMockProber{}
				prober2     = &simpleMockProber{workLoad: timeout * 2}
				sut         = unixcycle.ParallelProber(prober1.Probe, prober2.Probe)
			)
			defer cancel()

			// Act
			err := sut(ctx)

			// Assert
			require.Error(t, err)
			assert.ErrorContains(t, err, "parallel prober timed out")
			assert.Equal(t, 1, int(prober1.calls.Load()))
			assert.Equal(t, 0, int(prober2.calls.Load())) // prober2 should not have been called due to timeout
		})

		t.Run("should return nil if all probers succeed", func(t *testing.T) {
			t.Parallel()
			// Arrange
			var (
				prober1 = &simpleMockProber{}
				prober2 = &simpleMockProber{}
				sut     = unixcycle.ParallelProber(prober1.Probe, prober2.Probe)
			)

			// Act
			err := sut(context.Background())

			// Assert
			require.NoError(t, err)
			assert.Equal(t, 1, int(prober1.calls.Load()))
			assert.Equal(t, 1, int(prober2.calls.Load()))
		})
	})

	t.Run("ParallelRetryingProber", func(t *testing.T) {
		t.Parallel()

		t.Run("should retry probers until timeout", func(t *testing.T) {
			t.Parallel()
			// Arrange
			var (
				ctx, cancel = context.WithTimeout(context.Background(), timeout)
				prober1     = &simpleMockProber{successAfterCalls: maxAttempts - 1}
				prober2     = &simpleMockProber{successAfterCalls: maxAttempts + 1} // This one will never succeed
				sut         = unixcycle.ParallelProber(
					unixcycle.RetryingProber(retryDelay, timeout, prober1.Probe),
					unixcycle.RetryingProber(retryDelay, timeout, prober2.Probe),
				)
			)
			defer cancel()

			// Act
			err := sut(ctx)

			// Assert
			require.Error(t, err)
			assert.ErrorContains(t, err, "parallel prober timed out")
			assert.Equal(t, maxAttempts-1, int(prober1.calls.Load()))
			assert.Equal(t, maxAttempts, int(prober2.calls.Load())) // Should be equal to maxAttempts since that's when timeout hits
		})

		t.Run("should return error if one of the probers gives up", func(t *testing.T) {
			t.Parallel()
			// Arrange
			var (
				ctx, cancel = context.WithTimeout(context.Background(), timeout)
				prober1     = &simpleMockProber{successAfterCalls: maxAttempts}
				prober2     = &simpleMockProber{successAfterCalls: maxAttempts}
				sut         = unixcycle.ParallelProber(
					unixcycle.RetryingProber(retryDelay, timeout, prober1.Probe),
					unixcycle.RetryingProber(retryDelay, timeout/2, prober2.Probe), // This one will timeout hence give up
				)
			)
			defer cancel()

			// Act
			err := sut(ctx)

			// Assert
			require.Error(t, err)
			assert.ErrorContains(t, err, "parallel prober errored")
			assert.Equal(t, maxAttempts/2, int(prober1.calls.Load())) // should be equal to maxAttempts/2 since the parallel prober gives up when the first one fails
			assert.Equal(t, maxAttempts/2, int(prober2.calls.Load())) // Should be equal to maxAttempts/2 since timeout if halfed
		})

		t.Run("should not keep retrying a prober if it already succeeded", func(t *testing.T) {
			t.Parallel()
			// Arrange
			var (
				ctx, cancel = context.WithTimeout(context.Background(), timeout)
				prober1     = &simpleMockProber{successAfterCalls: maxAttempts}
				prober2     = &simpleMockProber{successAfterCalls: maxAttempts}
				sut         = unixcycle.ParallelProber(
					unixcycle.RetryingProber(retryDelay, timeout, prober1.Probe),
					unixcycle.RetryingProber(retryDelay/2, timeout/2, prober2.Probe), // finishes twice as fast, but we expect same amount of calls
				)
			)
			defer cancel()

			// Act
			err := sut(ctx)

			// Assert
			require.NoError(t, err)
			assert.Equal(t, maxAttempts, int(prober1.calls.Load()))
			assert.Equal(t, maxAttempts, int(prober2.calls.Load()))
		})
	})

	t.Run("TestMain", func(t *testing.T) {
		type dependencies struct {
			testingM     *TestingMMock
			manager      *unixcycle.Manager
			prober       *ProberMock
			testFixtures []unixcycle.Component
		}
		var (
			// newCtx  = func() context.Context { return context.Background() }
			newTestFixture = func() *componentMock { return &componentMock{} }
			newDeps        = func(mods ...func(deps *dependencies)) *dependencies {
				deps := &dependencies{
					testingM:     &TestingMMock{},
					manager:      unixcycle.NewManager(),
					prober:       &ProberMock{},
					testFixtures: make([]unixcycle.Component, 0),
				}
				deps.testingM.RunFunc = func() int {
					return 0
				}
				deps.prober.ProbeFunc = func(ctx context.Context) error {
					return nil
				}

				for _, mod := range mods {
					mod(deps)
				}

				return deps
			}
			newSut = func(deps *dependencies) func() int {
				return func() int {
					return unixcycle.TestMain(
						deps.testingM,
						deps.manager,
						unixcycle.ProberFunc(deps.prober.Probe),
						deps.testFixtures...,
					)
				}
			}
		)

		t.Run("should call test-fixture", func(t *testing.T) {
			t.Parallel()
			// Arrange
			var (
				expectedSignal = 8 // Some value
				deps           = newDeps()
				sut            = newSut(deps)
				testFixture    = newTestFixture()
			)
			deps.testingM.RunFunc = func() int { return expectedSignal }
			deps.testFixtures = append(deps.testFixtures, testFixture)
			deps.prober.ProbeFunc = unixcycle.RetryingProber(10*time.Millisecond, 2*time.Second, func(ctx context.Context) error {
				if testFixture.getSetupCalls() != 1 {
					return assert.AnError
				}
				return nil
			}).Probe

			// Act
			signal := sut()

			// Assert
			assert.Equal(t, expectedSignal, signal)
			assert.Equal(t, testFixture.getSetupCalls(), 1)
			assert.Equal(t, testFixture.getStartCalls(), 1)
			assert.Equal(t, testFixture.getCloseCalls(), 1)
			assert.Len(t, deps.testingM.RunCalls(), 1)
			assert.Len(t, deps.prober.ProbeCalls(), 1)
		})

		t.Run("should call m.Run and return signal", func(t *testing.T) {
			t.Parallel()
			// Arrange
			var (
				expectedSignal = 8
				deps           = newDeps()
				sut            = newSut(deps)
			)
			deps.testingM.RunFunc = func() int { return expectedSignal }

			// Act
			signal := sut()

			// Assert
			assert.Equal(t, expectedSignal, signal)
			assert.Len(t, deps.prober.ProbeCalls(), 1)
			assert.Len(t, deps.testingM.RunCalls(), 1)
		})

		t.Run("should not call m.Run if prober fails", func(t *testing.T) {
			t.Parallel()
			// Arrange
			var (
				expectedSignal = int(syscall.SIGUSR1)
				deps           = newDeps()
				sut            = newSut(deps)
			)
			deps.testingM.RunFunc = func() int { return expectedSignal }
			deps.prober.ProbeFunc = func(ctx context.Context) error {
				return assert.AnError
			}

			// Act
			signal := sut()

			// Assert
			assert.Equal(t, expectedSignal, signal)
			assert.Len(t, deps.testingM.RunCalls(), 0)
			assert.Len(t, deps.prober.ProbeCalls(), 1)
		})
	})
}

type simpleMockProber struct {
	successAfterCalls int
	workLoad          time.Duration

	calls atomic.Int64
}

func (s *simpleMockProber) Probe(ctx context.Context) error {
	time.Sleep(s.workLoad)
	s.calls.Add(1)
	if s.calls.Load() < int64(s.successAfterCalls) {
		return assert.AnError
	}
	return nil
}

type componentMock struct {
	mu        sync.Mutex
	setupFunc func() error
	startFunc func() error
	closeFunc func() error

	calls struct {
		setup []struct{}
		start []struct{}
		close []struct{}
	}
}

func (c *componentMock) Setup() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls.setup = append(c.calls.setup, struct{}{})
	if c.setupFunc == nil {
		return nil
	}
	return c.setupFunc()
}
func (c *componentMock) Start() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls.start = append(c.calls.start, struct{}{})
	if c.startFunc == nil {
		return nil
	}
	return c.startFunc()
}
func (c *componentMock) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls.close = append(c.calls.close, struct{}{})
	if c.closeFunc == nil {
		return nil
	}
	return c.closeFunc()
}
func (c *componentMock) getSetupCalls() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.calls.setup)
}
func (c *componentMock) getStartCalls() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.calls.start)
}
func (c *componentMock) getCloseCalls() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.calls.close)
}
