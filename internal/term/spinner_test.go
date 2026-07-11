package term

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

func TestSpinnerDoesNotWriteWhenStdoutIsNotTerminal(t *testing.T) {
	origStdout := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = writer
	defer func() {
		os.Stdout = origStdout
	}()

	var spinner Spinner
	spinner.Start("Generating commit message with codex...")
	time.Sleep(120 * time.Millisecond)
	spinner.Stop()

	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	var out bytes.Buffer
	if _, err := io.Copy(&out, reader); err != nil {
		t.Fatalf("read stdout pipe: %v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("spinner wrote to non-terminal stdout: %q", out.String())
	}
}

func TestSpinnerStopWithoutStartIsNoop(t *testing.T) {
	var spinner Spinner
	spinner.Stop()
}

func TestSpinnerContentFitsNarrowTerminal(t *testing.T) {
	const width = 40
	content := spinnerContent(spinnerFrames[0], "Generating commit message with codex · gpt-5.6-luna...", width)

	if got, max := displayWidth(content), safeTerminalWidth(width); got > max {
		t.Fatalf("spinner width = %d, want at most %d: %q", got, max, content)
	}
	if !strings.HasSuffix(content, "…") {
		t.Fatalf("truncated spinner content = %q, want ellipsis", content)
	}
}

func TestSpinnerContentPreservesLabelWhenItFits(t *testing.T) {
	const label = "Generating commit message..."
	content := spinnerContent(spinnerFrames[0], label, 80)

	if !strings.HasSuffix(content, label) {
		t.Fatalf("spinner content = %q, want complete label %q", content, label)
	}
}

func TestSpinnerContentFitsMinimalTerminalWidths(t *testing.T) {
	for width := 1; width <= 4; width++ {
		content := spinnerContent(spinnerFrames[0], "Generating commit message...", width)
		if got, max := displayWidth(content), safeTerminalWidth(width); got > max {
			t.Errorf("width %d: spinner width = %d, want at most %d: %q", width, got, max, content)
		}
	}
}
