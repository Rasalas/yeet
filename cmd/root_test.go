package cmd

import (
	"reflect"
	"testing"

	"github.com/rasalas/yeet/internal/git"
	"github.com/rasalas/yeet/internal/term"
	"github.com/spf13/cobra"
)

type runYeetMockGit struct {
	hasStagedChanges      bool
	diffStat              string
	commitOut             string
	currentBranch         string
	commitCalled          bool
	commitMessage         string
	pushCalled            bool
	pushSetUpstreamCalled bool
}

func (m *runYeetMockGit) HasStagedChanges() bool      { return m.hasStagedChanges }
func (m *runYeetMockGit) StageAll() error             { return nil }
func (m *runYeetMockGit) DiffStat() (string, error)   { return m.diffStat, nil }
func (m *runYeetMockGit) DiffCached() (string, error) { return "", nil }
func (m *runYeetMockGit) Commit(msg string) (string, error) {
	m.commitCalled = true
	m.commitMessage = msg
	return m.commitOut, nil
}
func (m *runYeetMockGit) Push() (string, error) { m.pushCalled = true; return "", nil }
func (m *runYeetMockGit) PushSetUpstream() (string, error) {
	m.pushSetUpstreamCalled = true
	return "", nil
}
func (m *runYeetMockGit) Reset() error                         { return nil }
func (m *runYeetMockGit) LogOneline() (string, error)          { return "", nil }
func (m *runYeetMockGit) StatusShort() (string, error)         { return "", nil }
func (m *runYeetMockGit) CurrentBranch() (string, error)       { return m.currentBranch, nil }
func (m *runYeetMockGit) DefaultBranch() (string, error)       { return "main", nil }
func (m *runYeetMockGit) LogRange(string) (string, error)      { return "", nil }
func (m *runYeetMockGit) DiffRange(string) (string, error)     { return "", nil }
func (m *runYeetMockGit) DiffStatRange(string) (string, error) { return "", nil }
func (m *runYeetMockGit) HasUpstream() bool                    { return true }

func TestFirstLine(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"single line", "single line"},
		{"first\nsecond\nthird", "first"},
		{"", ""},
		{"\nleading newline", ""},
		{"trailing newline\n", "trailing newline"},
	}

	for _, tt := range tests {
		got := firstLine(tt.input)
		if got != tt.want {
			t.Errorf("firstLine(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestRootCommandAcceptsPositionalMessage(t *testing.T) {
	origRunE := rootCmd.RunE
	rootCmd.SetArgs(nil)
	defer func() {
		rootCmd.RunE = origRunE
		rootCmd.SetArgs(nil)
	}()

	var got []string
	rootCmd.RunE = func(cmd *cobra.Command, args []string) error {
		got = append([]string(nil), args...)
		return nil
	}

	want := []string{"feat(gui): update svelte ui to html version"}
	rootCmd.SetArgs(want)
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("rootCmd.Execute() returned error: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("root command args = %q, want %q", got, want)
	}
}

func TestRunYeetWithLocalFlagSkipsPush(t *testing.T) {
	origGit := git.Default
	origMessageFlag := messageFlag
	origYesFlag := yesFlag
	origLocalFlag := localFlag
	defer func() {
		git.Default = origGit
		messageFlag = origMessageFlag
		yesFlag = origYesFlag
		localFlag = origLocalFlag
	}()

	mock := &runYeetMockGit{
		hasStagedChanges: true,
		diffStat:         " cmd/root.go | 3 ++-",
		commitOut:        "[main abc1234] feat: local commit",
		currentBranch:    "main",
	}
	git.Default = mock

	messageFlag = "feat: local commit"
	yesFlag = true
	localFlag = true

	if err := runYeet(rootCmd, nil); err != nil {
		t.Fatalf("runYeet returned error: %v", err)
	}

	if !mock.commitCalled {
		t.Fatal("expected commit to be called")
	}
	if mock.commitMessage != "feat: local commit" {
		t.Fatalf("commit message = %q, want %q", mock.commitMessage, "feat: local commit")
	}
	if mock.pushCalled {
		t.Fatal("expected push not to be called when --local is set")
	}
	if mock.pushSetUpstreamCalled {
		t.Fatal("expected push --set-upstream not to be called when --local is set")
	}
}

func TestMessageCardAndStreamLineCalculations(t *testing.T) {
	width := 20
	message := "123456789012345" // len = 15

	origMsgBg := term.MsgBg
	term.MsgBg = "bg"
	defer func() { term.MsgBg = origMsgBg }()

	if got := messageCardRows(message, width); got != 2 {
		t.Fatalf("messageCardRows() = %d, want %d", got, 2)
	}

	// Top + 2 wrapped rows + bottom = 4 visible lines.
	if got := streamedPreviewRenderedLines(message, width); got != 4 {
		t.Fatalf("streamedPreviewRenderedLines() = %d, want %d", got, 4)
	}
}

func TestRenderedBlockClearLines(t *testing.T) {
	width := 10
	origMsgBg := term.MsgBg
	term.MsgBg = ""
	defer func() { term.MsgBg = origMsgBg }()

	message := "1234567" // width-4 = 6 => 2 visible rows
	if got := term.RenderedBlockClearLines(term.PlainMessageRows(message, width)); got != 3 {
		t.Fatalf("RenderedBlockClearLines(plain message) = %d, want %d", got, 3)
	}
}

func TestStreamedPreviewRenderedLinesPlain(t *testing.T) {
	width := 10
	origMsgBg := term.MsgBg
	term.MsgBg = ""
	defer func() { term.MsgBg = origMsgBg }()

	message := "1234567" // width-4 = 6 => 2 visible rows
	if got := streamedPreviewRenderedLines(message, width); got != 2 {
		t.Fatalf("streamedPreviewRenderedLines() = %d, want %d", got, 2)
	}
}

func TestRenderedBlockClearLinesMultiple(t *testing.T) {
	if got := term.RenderedBlockClearLines(5, 2); got != 8 {
		t.Fatalf("RenderedBlockClearLines() = %d, want %d", got, 8)
	}
}
