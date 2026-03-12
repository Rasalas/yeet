package cmd

import (
	"reflect"
	"testing"

	"github.com/rasalas/yeet/internal/git"
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

func TestWrappedLineCount(t *testing.T) {
	tests := []struct {
		name    string
		columns int
		width   int
		want    int
	}{
		{name: "single row", columns: 10, width: 80, want: 1},
		{name: "exact width", columns: 20, width: 20, want: 1},
		{name: "wraps once", columns: 21, width: 20, want: 2},
		{name: "zero columns", columns: 0, width: 20, want: 1},
		{name: "invalid width fallback", columns: 81, width: 0, want: 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := wrappedLineCount(tt.columns, tt.width); got != tt.want {
				t.Fatalf("wrappedLineCount(%d, %d) = %d, want %d", tt.columns, tt.width, got, tt.want)
			}
		})
	}
}

func TestMessageCardAndStreamLineCalculations(t *testing.T) {
	width := 20
	message := "123456789012345" // len = 15

	if got := messageCardRows(message, width); got != 2 {
		t.Fatalf("messageCardRows() = %d, want %d", got, 2)
	}

	// Top + 2 wrapped rows + bottom + blank line = 5.
	if got := messageCardClearLines(message, width); got != 5 {
		t.Fatalf("messageCardClearLines() = %d, want %d", got, 5)
	}

	// Streamed preview wrapped rows (2) + current line (1) => 3.
	if got := streamedPreviewClearLines(message, width); got != 3 {
		t.Fatalf("streamedPreviewClearLines() = %d, want %d", got, 3)
	}
}

func TestPlainMessageClearLines(t *testing.T) {
	width := 10
	message := "1234567" // width-4 = 6 => 2 rows + blank = 3
	if got := plainMessageClearLines(message, width); got != 3 {
		t.Fatalf("plainMessageClearLines() = %d, want %d", got, 3)
	}
}

func TestPrintHintActionsWraps(t *testing.T) {
	width := 20
	actions := []hintAction{
		{key: "enter", desc: "commit"},
		{key: "e", desc: "edit"},
		{key: "E", desc: "editor"},
	}

	if got := printHintActions(actions, width); got != 3 {
		t.Fatalf("printHintActions() = %d, want %d", got, 3)
	}
}
