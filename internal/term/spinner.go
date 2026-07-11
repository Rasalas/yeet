package term

import (
	"fmt"
	"strings"
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
			printSpinnerFrame(spinnerFrames[0], label)
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
					printSpinnerFrame(spinnerFrames[i], label)
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

func printSpinnerFrame(frame rune, label string) {
	fmt.Printf("\r\033[K%s%s%s", Dim, spinnerContent(frame, label, TerminalWidth()), Reset)
}

func spinnerContent(frame rune, label string, width int) string {
	available := safeTerminalWidth(width)
	frameText := string(frame)
	if available == 1 {
		return frameText
	}
	if available == 2 {
		return " " + frameText
	}

	prefix := "  " + frameText
	labelWidth := available - displayWidth(prefix) - 1
	if labelWidth < 1 {
		return prefix
	}
	return prefix + " " + truncateSpinnerLabel(label, labelWidth)
}

func truncateSpinnerLabel(label string, width int) string {
	if width <= 0 || label == "" {
		return ""
	}
	if displayWidth(label) <= width {
		return label
	}
	if width == 1 {
		return "…"
	}

	var truncated strings.Builder
	used := 0
	for _, r := range label {
		runeWidth := runeDisplayWidth(r)
		if used+runeWidth > width-1 {
			break
		}
		truncated.WriteRune(r)
		used += runeWidth
	}
	truncated.WriteRune('…')
	return truncated.String()
}
