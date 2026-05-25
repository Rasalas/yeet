package term

import (
	"fmt"
	"sync"
	"time"
)

var spinnerFrames = []rune{'\u280b', '\u2819', '\u2839', '\u2838', '\u283c', '\u2834', '\u2826', '\u2827', '\u2807', '\u280f'}

// Spinner displays a braille animation while waiting.
type Spinner struct {
	mu        sync.Mutex
	done      chan struct{}
	finished  chan struct{}
	stopped   bool
	firstText bool // true once the caller has replaced the spinner with content
	active    bool
}

// Start begins the spinner animation with the given label.
func (s *Spinner) Start(label string) {
	s.Stop()

	if !IsInteractive() {
		return
	}

	done := make(chan struct{})
	finished := make(chan struct{})

	s.mu.Lock()
	s.done = done
	s.finished = finished
	s.stopped = false
	s.firstText = false
	s.active = true
	s.mu.Unlock()

	go func() {
		defer close(finished)

		i := 0
		ticker := time.NewTicker(80 * time.Millisecond)
		defer ticker.Stop()

		s.mu.Lock()
		if !s.firstText {
			fmt.Printf("\r  %s%c %s%s", Dim, spinnerFrames[0], label, Reset)
		}
		s.mu.Unlock()

		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				s.mu.Lock()
				if !s.firstText {
					i = (i + 1) % len(spinnerFrames)
					fmt.Printf("\r  %s%c %s%s", Dim, spinnerFrames[i], label, Reset)
				}
				s.mu.Unlock()
			}
		}
	}()
}

// Stop ends the spinner animation and cleans up the line.
func (s *Spinner) Stop() {
	s.mu.Lock()
	if s.stopped || !s.active {
		s.mu.Unlock()
		return
	}

	done := s.done
	finished := s.finished
	s.stopped = true
	s.active = false
	s.done = nil
	s.finished = nil
	close(done)
	s.mu.Unlock()

	if finished != nil {
		<-finished
	}
	fmt.Print("\r\033[K")
}
