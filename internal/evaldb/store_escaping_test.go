package evaldb

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// hostileDiff collects inputs that have historically broken hand-rolled SQL
// escaping: quotes, backslashes, unicode, and SQL keywords that must stay
// inert data.
func hostileDiff() string {
	return strings.Join([]string{
		"diff --git a/x.go b/x.go",
		"-it's a trap'; DROP TABLE runs; --",
		`+msg := "quote \" inside"; path = 'C:\Users\x'`,
		"+emoji 🚀 ümlaut — dash",
		"+SELECT * FROM eval_variants WHERE ''='",
		"+backslash at end \\",
		"\\ No newline at end of file",
	}, "\n")
}

func TestSQLTextEscapesHostileContent(t *testing.T) {
	cases := []struct {
		name  string
		input string
	}{
		{"single quote", "it's"},
		{"two quotes", "a''b"},
		{"sql injection attempt", "x'; DROP TABLE runs; --"},
		{"backslashes", `C:\Users\test`},
		{"unicode", "äöü 🚀 —"},
		{"empty", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := sqlText(tc.input)
			if !strings.HasPrefix(got, "'") || !strings.HasSuffix(got, "'") {
				t.Fatalf("sqlText(%q) not quoted: %q", tc.input, got)
			}
			inner := got[1 : len(got)-1]
			if strings.Count(inner, "'")%2 != 0 {
				t.Errorf("unbalanced quotes in %q (input %q)", inner, tc.input)
			}
		})
	}
}

func newTestStore(t *testing.T) *Store {
	t.Helper()
	if _, err := exec.LookPath("sqlite3"); err != nil {
		t.Skip("sqlite3 not available on PATH")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")

	s := &Store{path: path}
	if err := s.exec(schemaSQL); err != nil {
		t.Fatalf("schema: %v", err)
	}
	return s
}

// TestStoreRoundTripsHostileDiff is the end-to-end guard for the escaper:
// whatever goes in as diff/message text must come back byte-identical, and
// the statement must not corrupt neighboring columns.
func TestStoreRoundTripsHostileDiff(t *testing.T) {
	s := newTestStore(t)

	diff := hostileDiff()
	msg := "fix(auth): it's a 'quoted' commit"
	err := s.InsertRun(RunRecord{
		Command:      "commit",
		Diff:         diff,
		Status:       "M x.go",
		Branch:       "feat/test'",
		AIMessage:    msg,
		FinalMessage: msg,
	})
	if err != nil {
		t.Fatalf("InsertRun: %v", err)
	}

	runs, err := s.SelectEligibleRuns(10, false, "")
	if err != nil {
		t.Fatalf("SelectEligibleRuns: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("got %d runs, want 1", len(runs))
	}
	if runs[0].Diff != diff {
		t.Errorf("diff round-trip mismatch:\n got %q\nwant %q", runs[0].Diff, diff)
	}
	if runs[0].AIMessage != msg {
		t.Errorf("message round-trip mismatch: %q", runs[0].AIMessage)
	}
}

func TestPhraseReportWithQuoteInPhrase(t *testing.T) {
	s := newTestStore(t)

	err := s.InsertRun(RunRecord{
		Command:   "commit",
		Diff:      hostileDiff(),
		AIMessage: "avoid saying for improved user",
	})
	if err != nil {
		t.Fatalf("InsertRun: %v", err)
	}

	stats, err := s.PhraseReport(0, "it's")
	if err != nil {
		t.Fatalf("PhraseReport with quote failed: %v", err)
	}
	_ = stats

	// The variant-id filter uses %d formatting; ensure no injection via
	// phrase text breaks the query.
	if _, err := s.PhraseReport(0, "for improved user"); err != nil {
		t.Errorf("PhraseReport plain phrase failed: %v", err)
	}
}
