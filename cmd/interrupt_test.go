package cmd

import (
	"os"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

func TestWaitForSignalOrDoneRunsFnOnSignal(t *testing.T) {
	ch := make(chan os.Signal, 1)
	done := make(chan struct{})

	var ran atomic.Bool
	go waitForSignalOrDone(ch, done, func() { ran.Store(true) })

	ch <- syscall.SIGINT

	deadline := time.After(2 * time.Second)
	for !ran.Load() {
		select {
		case <-deadline:
			t.Fatal("fn was not invoked on signal")
		default:
			time.Sleep(time.Millisecond)
		}
	}
}

func TestWaitForSignalOrDoneIgnoresClosedDone(t *testing.T) {
	ch := make(chan os.Signal, 1)
	done := make(chan struct{})
	close(done)

	var ran atomic.Bool
	finished := make(chan struct{})
	go func() {
		waitForSignalOrDone(ch, done, func() { ran.Store(true) })
		close(finished)
	}()

	select {
	case <-finished:
	case <-time.After(2 * time.Second):
		t.Fatal("waitForSignalOrDone did not return on closed done")
	}
	if ran.Load() {
		t.Error("fn must not run when done closes first")
	}
}
