package unixcycle

import (
	"context"
	"fmt"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"
)

type TestingM interface {
	Run() int
}

type Prober interface {
	Probe(ctx context.Context) error
}

type ProberFunc func(ctx context.Context) error

func (p ProberFunc) Probe(ctx context.Context) error {
	return p(ctx)
}

// TestMain is a test entry point that sets up a service.
// It instructs the manager with the test fixtures and run the prober.
// Whenever the prober gives green light, the tests are run.
// Great for acceptance tests, where you want to setup some fixtures (usually mocks) and run the tests.
func TestMain(m TestingM, manager *Manager, prober ProberFunc, testFixtures ...Component) int {
	var (
		managerStopped = make(chan int)
		proberLifetime = func() int {
			if err := prober(context.Background()); err != nil {
				manager.logError("unable to run tests due to prober failing with error", "error", err)
				return int(syscall.SIGUSR1)
			}
			return m.Run()
		}
	)
	manager.lifetime = proberLifetime

	for _, component := range testFixtures {
		manager.Add(fmt.Sprintf("test-fixture-%T", component), component)
	}

	go func() {
		sysSignal := manager.Run()
		managerStopped <- sysSignal
	}()

	return int(<-managerStopped)
}

func RetryingProber(retryDelay time.Duration, timeout time.Duration, prober ProberFunc) ProberFunc {
	return func(ctx context.Context) error {
		var (
			newCtx, cancel = context.WithTimeout(ctx, timeout)
			t              = time.NewTicker(retryDelay)
		)
		defer cancel()
		defer t.Stop()

		for {
			select {
			case <-newCtx.Done():
				if newCtx.Err() == context.DeadlineExceeded {
					return fmt.Errorf("prober timed out: %w", newCtx.Err())
				}
				return fmt.Errorf("retrying prober failed: %w", newCtx.Err())
			case tick := <-t.C:
				attemptCtx, attemptCancel := context.WithTimeout(newCtx, time.Until(tick.Add(retryDelay)))
				defer attemptCancel()

				if err := prober(attemptCtx); err != nil {
					continue // retry
				}
				return nil // success
			}
		}
	}
}

func ParallelProber(probers ...ProberFunc) ProberFunc {
	return func(ctx context.Context) error {
		var (
			errGroup, newCtx = errgroup.WithContext(ctx)
			errChan          = make(chan error)
		)
		for _, probe := range probers {
			errGroup.Go(func() error {
				return probe(newCtx)
			})
		}
		go func() {
			errChan <- errGroup.Wait()
		}()
		select {
		case err := <-errChan:
			if err != nil {
				return fmt.Errorf("parallel prober errored: %w", err)
			}
			return nil
		case <-ctx.Done():
			if ctx.Err() == context.DeadlineExceeded {
				return fmt.Errorf("parallel prober timed out: %w", newCtx.Err())
			}
			return fmt.Errorf("parallel prober failed: %w", newCtx.Err())
		}
	}
}
