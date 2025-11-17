package unixcycle

import (
	"os"
	"os/signal"
	"syscall"
)

type TerminationSignal func() int

func InterruptSignal() int {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	<-signals

	return 0
}
