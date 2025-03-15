package unixcycle

import (
	"os"
	"os/signal"
	"syscall"
)

type TerminationSignal func() syscall.Signal

func InterruptSignal() syscall.Signal {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	<-signals

	return syscall.Signal(0)
}
