package term

import (
	"bytes"
	"io"
	"os"
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
