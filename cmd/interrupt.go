package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/rasalas/yeet/internal/term"
)

// onInterrupt invokes fn on the first SIGINT/SIGTERM until the returned stop
// function is called. Used to undo yeet's auto-staging when the user aborts
// mid-generation, matching what Escape does in the confirmation loop.
func onInterrupt(fn func()) (stop func()) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	done := make(chan struct{})

	go waitForSignalOrDone(ch, done, fn)

	return func() {
		close(done)
		signal.Stop(ch)
	}
}

// waitForSignalOrDone runs fn once a signal arrives; it returns silently
// when done closes first.
func waitForSignalOrDone(ch <-chan os.Signal, done <-chan struct{}, fn func()) {
	select {
	case <-ch:
		fmt.Printf("\n  %sInterrupted.%s\n", term.Dim, term.Reset)
		fn()
	case <-done:
	}
}
