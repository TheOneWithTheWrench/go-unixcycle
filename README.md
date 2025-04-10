# Unixcycle

A lifecycle management library for Go applications. Unixcycle helps you manage the startup, concurrent execution, and graceful shutdown of different parts (components) of your application.

## 🤔 Motivation

Managing the lifecycle of concurrent components (like servers, workers, background tasks) in a Go application can become cumbersome and require a lot of boilerplate code. 
Ensuring proper initialization, concurrent operation, and clean shutdown in response to OS signals requires careful orchestration.

Unixcycle provides a structured and centralized way to handle this:

* Define independent components, each needing at least a `Start` method.
* Optionally define `Setup` for initialization and `Close` for cleanup.
* Ensure components are initialized (`Setup`).
* Run components concurrently (`Start`).
* Wait for termination signals (like `SIGINT`, `SIGTERM`).
* Gracefully shut down components (`Close`) in the correct order.

## ✨ Features

* **Component Lifecycle Management:** Manages components that implement `Start() error`, and optionally `Setup() error` and `Close() error`.
* **Concurrent Execution:** Runs each component's `Start` method in its own goroutine.
* **Ordered Operations:** Executes `Setup` methods sequentially in the order added, and `Close` methods sequentially in the *reverse* order added.
* **Graceful Shutdown:** Listens for OS termination signals (`SIGINT`, `SIGTERM` by default) to initiate shutdown.
* **Configurable Timeouts:** Set deadlines for `Setup` and `Close` operations to prevent hangs.
* **Customizable Logging:** Integrates with `slog`. Provide your own `slog.Handler`.
* **Flexible Component Definition:** Components primarily need `Start()`. `Setup` and `Close` are detected via optional interface implementation (`setupable`, `closable`).
* **Helper Functions:** Provides convenient helpers (`Starter`, `Setup`, `Closer`, `Make`) for creating components from functions or structs.
* **Clear Signal Handling:** Returns the `syscall.Signal` that triggered the shutdown, allowing for specific exit code logic.

## 💾 Installation

    go get github.com/TheOneWithTheWrench/go-unixcycle

## 🚀 Usage

Here's a simplified example showing a struct-based component and a function-based component:

```go
package main

import (
	"fmt"
	"log/slog"
	"os"
	"syscall"
	"time"

	unixcycle "github.com/TheOneWithTheWrench/go-unixcycle"
)

// Component 1: A service implementing Setup, Start, and Close
type MyService struct {
	stopCh chan struct{}
}

func NewMyService() *MyService {
	return &MyService{stopCh: make(chan struct{})}
}

// Setup is optional initialization logic.
func (s *MyService) Setup() error {
	time.Sleep(50 * time.Millisecond) // Simulate work
	return nil
}

// Start is required (unixcycle.Component interface). It should block.
func (s *MyService) Start() error {
	<-s.stopCh // Block until Close signals
	return nil
}

// Close is optional cleanup logic.
func (s *MyService) Close() error {
	close(s.stopCh) // Signal Start to exit
	time.Sleep(100 * time.Millisecond) // Simulate cleanup
	return nil
}

func main() {
	// Setup logger
	logHandler := slog.NewTextHandler(os.Stdout, nil)

	// Configure the manager
	manager := unixcycle.NewManager(
		unixcycle.WithLoggingHandler(logHandler),
		unixcycle.WithSetupTimeout(1*time.Second),
		unixcycle.WithCloseTimeout(2*time.Second),
		// unixcycle.WithLifetime(unixcycle.InterruptSignal), // Default
	)

	// Add components using helpers
	manager.
		// Use Make for struct pointers. It checks for Setup/Start/Close methods.
		Add("MyService", unixcycle.Make[MyService](NewMyService())).

	// Run blocks until a signal or error occurs.
	signal := manager.Run()

	os.Exit(exitCode)
}
```

## 📖 API Overview

### Manager

* `unixcycle.NewManager(options ...managerOption) *Manager`: Creates a new lifecycle manager. Accepts functional options for configuration.
* `manager.Add(name string, component Component) *Manager`: Registers a component. The `name` is for logging. `component` must satisfy the `unixcycle.Component` interface.
* `manager.Run() syscall.Signal`: Starts the managed lifecycle:
    1.  Calls `Setup()` sequentially on components implementing `setupable`.
    2.  Calls `Start()` concurrently on all components.
    3.  Waits for a termination signal (via `Lifetime` option).
    4.  Calls `Close()` sequentially (in reverse add order) on components implementing `closable`.
    * Returns the `syscall.Signal` causing shutdown or indicating an error (`SIGALRM` for timeout, `SIGABRT` for setup/close error).

### Core Interfaces

* `unixcycle.Component`: The minimum interface a component must satisfy.
    ```go
    type Component interface {
        Start() error
    }
    ```
* `unixcycle.setupable`: Optional interface for setup logic.
    ```go
    type setupable interface {
        Setup() error
    }
    ```
* `unixcycle.closable`: Optional interface for cleanup logic.
    ```go
    type closable interface {
        Close() error
    }
    ```
    The manager uses type assertions to check if a registered `Component` also implements `setupable` or `closable`.

### Helper Functions

These simplify creating `Component` values:

* `unixcycle.Make[T](*T)`: Takes a pointer to a struct (`*T`). The struct *must* implement `Start() error`. If it also implements `Setup()` and/or `Close()`, those methods will be used. This is the preferred way to add struct-based components.
* `unixcycle.Starter(func() error)`: Wraps a function to create a `Component` whose `Start()` method executes the function. It has no `Setup` or `Close` behavior.
* `unixcycle.Setup(func() error)`: Wraps a function to create a `Component` whose `Setup()` method executes the function. Its `Start()` is a no-op. It has no `Close` behavior. Useful for initialization-only tasks.
* `unixcycle.Closer(func() error)`: Wraps a function to create a `Component` whose `Close()` method executes the function. Its `Start()` is a no-op. It has no `Setup` behavior. Useful for cleanup-only tasks run at the end.

### Configuration Options

Pass these to `NewManager` using the `With...` functions:

* `unixcycle.WithLoggingHandler(handler slog.Handler)`: Sets the `slog` handler for logging. If `nil`, logging is disabled (sent to `io.Discard`). Defaults to a text handler writing to `os.Stdout`.
* `unixcycle.WithSetupTimeout(time.Duration)`: Timeout for *each* component's `Setup()` call. Defaults to 5 seconds.
* `unixcycle.WithCloseTimeout(time.Duration)`: Timeout for *each* component's `Close()` call. Defaults to 5 seconds.
* `unixcycle.WithLifetime(unixcycle.TerminationSignal)`: A function `func() syscall.Signal` that blocks until termination is requested. Defaults to `unixcycle.InterruptSignal` (waits for `SIGINT` or `SIGTERM`).

## ⚠️ Error Handling and Signals

* **Setup/Close Errors:** If `Setup` or `Close` returns an error, the manager stops immediately, skips subsequent steps in that phase, and `Run()` returns `syscall.SIGABRT`.
* **Setup/Close Timeouts:** If `Setup` or `Close` exceeds its timeout, the manager stops, and `Run()` returns `syscall.SIGALRM`.
* **Start Errors:** Errors returned from `Start()` are logged. They **do not** automatically stop other components or trigger manager shutdown. The goroutine for the failing component exits. Implement cross-component error handling if needed (e.g., using shared channels or context cancellation propagated from the manager).
* **Termination Signals:** `SIGINT`/`SIGTERM` (by default) trigger graceful shutdown. `Run()` returns the received signal.

## 🤝 Contributing

Contributions are welcome! Please feel free to submit issues and pull requests.

## 🏗️ Future work
* **More Tests:** Add more tests to ensure reliability and robustness.
* **Better testing setup:** Setup the manager with TestMain and probably have some concepts of "probes" that can answer when the manager is ready.

## 📜 License

This project is licensed under the MIT License - see the LICENSE file for details.
