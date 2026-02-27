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
	stopped   bool
	firstText bool // true once the caller has replaced the spinner with content
}

// Start begins the spinner animation with the given label.
func (s *Spinner) Start(label string) {
	s.done = make(chan struct{})
	s.stopped = false
	s.firstText = false

	go func() {
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
			case <-s.done:
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

// ReplaceWithContent clears the spinner and begins inline content.
// Call this on the first token. Returns true if this was the first call.
func (s *Spinner) ReplaceWithContent(content string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.firstText {
		s.firstText = true
		fmt.Printf("\r\033[K")
		fmt.Printf("  %s%s", MsgOpen, content)
		return true
	}
	fmt.Print(content)
	return false
}

// Stop ends the spinner animation and cleans up the line.
func (s *Spinner) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.stopped {
		s.stopped = true
		close(s.done)
		if s.firstText {
			fmt.Printf("%s\n", MsgClose)
		} else {
			fmt.Print("\r\033[K")
		}
	}
}
