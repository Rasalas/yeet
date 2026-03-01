package evaldb

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/rasalas/yeet/internal/xdg"
)

const schemaSQL = `
CREATE TABLE IF NOT EXISTS runs (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	created_at TEXT NOT NULL,
	repo_path TEXT NOT NULL,
	command TEXT NOT NULL,
	branch TEXT NOT NULL,
	status TEXT NOT NULL,
	recent_commits TEXT NOT NULL,
	diff TEXT NOT NULL,
	provider TEXT NOT NULL,
	model TEXT NOT NULL,
	prompt_hash TEXT NOT NULL,
	prompt_text TEXT NOT NULL,
	ai_message TEXT NOT NULL,
	final_message TEXT NOT NULL,
	user_action TEXT NOT NULL,
	input_tokens INTEGER NOT NULL DEFAULT 0,
	output_tokens INTEGER NOT NULL DEFAULT 0,
	cost_usd REAL NOT NULL DEFAULT 0,
	latency_ms INTEGER NOT NULL DEFAULT 0,
	local_only INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_runs_created_at ON runs(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_runs_command ON runs(command);

CREATE TABLE IF NOT EXISTS eval_variants (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	created_at TEXT NOT NULL,
	provider TEXT NOT NULL,
	model TEXT NOT NULL,
	prompt_hash TEXT NOT NULL,
	prompt_text TEXT NOT NULL,
	prompt_path TEXT NOT NULL,
	sample_limit INTEGER NOT NULL,
	batch_size INTEGER NOT NULL,
	max_cost_usd REAL NOT NULL,
	edited_only INTEGER NOT NULL DEFAULT 0,
	contains_text TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_eval_variants_created_at ON eval_variants(created_at DESC);

CREATE TABLE IF NOT EXISTS eval_candidates (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	created_at TEXT NOT NULL,
	variant_id INTEGER NOT NULL,
	run_id INTEGER NOT NULL,
	message TEXT NOT NULL,
	input_tokens INTEGER NOT NULL DEFAULT 0,
	output_tokens INTEGER NOT NULL DEFAULT 0,
	cost_usd REAL NOT NULL DEFAULT 0,
	latency_ms INTEGER NOT NULL DEFAULT 0,
	error TEXT NOT NULL DEFAULT '',
	FOREIGN KEY(variant_id) REFERENCES eval_variants(id) ON DELETE CASCADE,
	FOREIGN KEY(run_id) REFERENCES runs(id) ON DELETE CASCADE,
	UNIQUE(variant_id, run_id)
);
CREATE INDEX IF NOT EXISTS idx_eval_candidates_variant ON eval_candidates(variant_id);

CREATE TABLE IF NOT EXISTS eval_votes (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	created_at TEXT NOT NULL,
	candidate_id INTEGER NOT NULL UNIQUE,
	winner TEXT NOT NULL,
	display_left TEXT NOT NULL,
	display_right TEXT NOT NULL,
	FOREIGN KEY(candidate_id) REFERENCES eval_candidates(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_eval_votes_candidate ON eval_votes(candidate_id);
`

type Store struct {
	path string
}

type RunRecord struct {
	CreatedAt     time.Time
	RepoPath      string
	Command       string
	Branch        string
	Status        string
	RecentCommits string
	Diff          string
	Provider      string
	Model         string
	PromptHash    string
	PromptText    string
	AIMessage     string
	FinalMessage  string
	UserAction    string
	InputTokens   int
	OutputTokens  int
	CostUSD       float64
	LatencyMS     int64
	LocalOnly     bool
}

type Run struct {
	ID            int64   `json:"id"`
	CreatedAt     string  `json:"created_at"`
	Branch        string  `json:"branch"`
	Status        string  `json:"status"`
	RecentCommits string  `json:"recent_commits"`
	Diff          string  `json:"diff"`
	Provider      string  `json:"provider"`
	Model         string  `json:"model"`
	PromptHash    string  `json:"prompt_hash"`
	PromptText    string  `json:"prompt_text"`
	AIMessage     string  `json:"ai_message"`
	FinalMessage  string  `json:"final_message"`
	InputTokens   int     `json:"input_tokens"`
	OutputTokens  int     `json:"output_tokens"`
	CostUSD       float64 `json:"cost_usd"`
}

type VariantSpec struct {
	Provider    string
	Model       string
	PromptHash  string
	PromptText  string
	PromptPath  string
	SampleLimit int
	BatchSize   int
	MaxCostUSD  float64
	EditedOnly  bool
	Contains    string
}

type Variant struct {
	ID          int64   `json:"id"`
	CreatedAt   string  `json:"created_at"`
	Provider    string  `json:"provider"`
	Model       string  `json:"model"`
	PromptHash  string  `json:"prompt_hash"`
	PromptText  string  `json:"prompt_text"`
	PromptPath  string  `json:"prompt_path"`
	SampleLimit int     `json:"sample_limit"`
	BatchSize   int     `json:"batch_size"`
	MaxCostUSD  float64 `json:"max_cost_usd"`
	EditedOnly  int     `json:"edited_only"`
	Contains    string  `json:"contains_text"`
}

type CandidateRecord struct {
	VariantID    int64
	RunID        int64
	Message      string
	InputTokens  int
	OutputTokens int
	CostUSD      float64
	LatencyMS    int64
	Error        string
}

type JudgeItem struct {
	CandidateID      int64  `json:"candidate_id"`
	RunID            int64  `json:"run_id"`
	RunCreatedAt     string `json:"run_created_at"`
	BaselineMessage  string `json:"baseline_message"`
	CandidateMessage string `json:"candidate_message"`
}

type Report struct {
	TotalCandidates     int     `json:"total_candidates"`
	GeneratedCandidates int     `json:"generated_candidates"`
	FailedCandidates    int     `json:"failed_candidates"`
	JudgedCandidates    int     `json:"judged_candidates"`
	CandidateWins       int     `json:"candidate_wins"`
	BaselineWins        int     `json:"baseline_wins"`
	Ties                int     `json:"ties"`
	BothBad             int     `json:"both_bad"`
	CandidateAvgCost    float64 `json:"candidate_avg_cost"`
	BaselineAvgCost     float64 `json:"baseline_avg_cost"`
	CandidateAvgLatency float64 `json:"candidate_avg_latency"`
	BaselineAvgLatency  float64 `json:"baseline_avg_latency"`
}

type PhraseStats struct {
	BaselineHits  int `json:"baseline_hits"`
	CandidateHits int `json:"candidate_hits"`
	Compared      int `json:"compared"`
}

func DBPath() (string, error) {
	dataDir, err := xdg.DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dataDir, "yeet", "yeet.db"), nil
}

func Open() (*Store, error) {
	dbPath, err := DBPath()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, err
	}

	s := &Store{path: dbPath}
	if err := s.exec(schemaSQL); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) Path() string {
	return s.path
}

func (s *Store) InsertRun(r RunRecord) error {
	createdAt := r.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}

	sql := fmt.Sprintf(`
INSERT INTO runs (
	created_at, repo_path, command, branch, status, recent_commits, diff,
	provider, model, prompt_hash, prompt_text,
	ai_message, final_message, user_action,
	input_tokens, output_tokens, cost_usd, latency_ms, local_only
) VALUES (
	%s, %s, %s, %s, %s, %s, %s,
	%s, %s, %s, %s,
	%s, %s, %s,
	%d, %d, %s, %d, %d
);`,
		sqlText(createdAt.Format(time.RFC3339Nano)),
		sqlText(r.RepoPath),
		sqlText(r.Command),
		sqlText(r.Branch),
		sqlText(r.Status),
		sqlText(r.RecentCommits),
		sqlText(r.Diff),
		sqlText(r.Provider),
		sqlText(r.Model),
		sqlText(r.PromptHash),
		sqlText(r.PromptText),
		sqlText(r.AIMessage),
		sqlText(r.FinalMessage),
		sqlText(r.UserAction),
		r.InputTokens,
		r.OutputTokens,
		sqlFloat(r.CostUSD),
		r.LatencyMS,
		boolAsInt(r.LocalOnly),
	)

	return s.exec(sql)
}

func (s *Store) SelectEligibleRuns(limit int, editedOnly bool, contains string) ([]Run, error) {
	if limit <= 0 {
		limit = 20
	}

	clauses := []string{
		"command = 'commit'",
		"ai_message <> ''",
		"diff <> ''",
	}
	if editedOnly {
		clauses = append(clauses, "final_message <> ai_message")
	}
	if contains != "" {
		clauses = append(clauses, fmt.Sprintf("INSTR(LOWER(ai_message), LOWER(%s)) > 0", sqlText(contains)))
	}

	sql := fmt.Sprintf(`
SELECT
	id, created_at, branch, status, recent_commits, diff,
	provider, model, prompt_hash, prompt_text,
	ai_message, final_message,
	input_tokens, output_tokens, cost_usd
FROM runs
WHERE %s
ORDER BY created_at DESC
LIMIT %d;`, strings.Join(clauses, " AND "), limit)

	var rows []Run
	if err := s.query(sql, &rows); err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *Store) CreateVariant(spec VariantSpec) (int64, error) {
	if spec.SampleLimit <= 0 {
		spec.SampleLimit = 20
	}
	if spec.BatchSize <= 0 {
		spec.BatchSize = 5
	}

	sql := fmt.Sprintf(`
INSERT INTO eval_variants (
	created_at, provider, model, prompt_hash, prompt_text, prompt_path,
	sample_limit, batch_size, max_cost_usd, edited_only, contains_text
) VALUES (
	%s, %s, %s, %s, %s, %s,
	%d, %d, %s, %d, %s
);
SELECT last_insert_rowid() AS id;`,
		sqlText(time.Now().UTC().Format(time.RFC3339Nano)),
		sqlText(spec.Provider),
		sqlText(spec.Model),
		sqlText(spec.PromptHash),
		sqlText(spec.PromptText),
		sqlText(spec.PromptPath),
		spec.SampleLimit,
		spec.BatchSize,
		sqlFloat(spec.MaxCostUSD),
		boolAsInt(spec.EditedOnly),
		sqlText(spec.Contains),
	)

	var ids []struct {
		ID int64 `json:"id"`
	}
	if err := s.query(sql, &ids); err != nil {
		return 0, err
	}
	if len(ids) == 0 {
		return 0, fmt.Errorf("failed to create eval variant")
	}
	return ids[0].ID, nil
}

func (s *Store) InsertCandidate(c CandidateRecord) error {
	sql := fmt.Sprintf(`
INSERT OR REPLACE INTO eval_candidates (
	created_at, variant_id, run_id, message,
	input_tokens, output_tokens, cost_usd, latency_ms, error
) VALUES (
	%s, %d, %d, %s, %d, %d, %s, %d, %s
);`,
		sqlText(time.Now().UTC().Format(time.RFC3339Nano)),
		c.VariantID,
		c.RunID,
		sqlText(c.Message),
		c.InputTokens,
		c.OutputTokens,
		sqlFloat(c.CostUSD),
		c.LatencyMS,
		sqlText(c.Error),
	)
	return s.exec(sql)
}

func (s *Store) LatestVariant() (Variant, bool, error) {
	sql := `
SELECT
	id, created_at, provider, model, prompt_hash, prompt_text, prompt_path,
	sample_limit, batch_size, max_cost_usd, edited_only, contains_text
FROM eval_variants
ORDER BY id DESC
LIMIT 1;`
	var rows []Variant
	if err := s.query(sql, &rows); err != nil {
		return Variant{}, false, err
	}
	if len(rows) == 0 {
		return Variant{}, false, nil
	}
	return rows[0], true, nil
}

func (s *Store) GetVariant(variantID int64) (Variant, bool, error) {
	sql := fmt.Sprintf(`
SELECT
	id, created_at, provider, model, prompt_hash, prompt_text, prompt_path,
	sample_limit, batch_size, max_cost_usd, edited_only, contains_text
FROM eval_variants
WHERE id = %d
LIMIT 1;`, variantID)
	var rows []Variant
	if err := s.query(sql, &rows); err != nil {
		return Variant{}, false, err
	}
	if len(rows) == 0 {
		return Variant{}, false, nil
	}
	return rows[0], true, nil
}

func (s *Store) NextUnvotedCandidate(variantID int64) (JudgeItem, bool, error) {
	sql := fmt.Sprintf(`
SELECT
	c.id AS candidate_id,
	c.run_id AS run_id,
	r.created_at AS run_created_at,
	r.ai_message AS baseline_message,
	c.message AS candidate_message
FROM eval_candidates c
JOIN runs r ON r.id = c.run_id
LEFT JOIN eval_votes v ON v.candidate_id = c.id
WHERE c.variant_id = %d
  AND c.error = ''
  AND v.id IS NULL
ORDER BY c.id ASC
LIMIT 1;`, variantID)
	var rows []JudgeItem
	if err := s.query(sql, &rows); err != nil {
		return JudgeItem{}, false, err
	}
	if len(rows) == 0 {
		return JudgeItem{}, false, nil
	}
	return rows[0], true, nil
}

func (s *Store) SaveVote(candidateID int64, winner, displayLeft, displayRight string) error {
	sql := fmt.Sprintf(`
INSERT INTO eval_votes (created_at, candidate_id, winner, display_left, display_right)
VALUES (%s, %d, %s, %s, %s)
ON CONFLICT(candidate_id) DO UPDATE SET
	created_at = excluded.created_at,
	winner = excluded.winner,
	display_left = excluded.display_left,
	display_right = excluded.display_right;`,
		sqlText(time.Now().UTC().Format(time.RFC3339Nano)),
		candidateID,
		sqlText(winner),
		sqlText(displayLeft),
		sqlText(displayRight),
	)
	return s.exec(sql)
}

func (s *Store) PendingCount(variantID int64) (int, error) {
	sql := fmt.Sprintf(`
SELECT COUNT(*) AS count
FROM eval_candidates c
LEFT JOIN eval_votes v ON v.candidate_id = c.id
WHERE c.variant_id = %d
  AND c.error = ''
  AND v.id IS NULL;`, variantID)
	var rows []struct {
		Count int `json:"count"`
	}
	if err := s.query(sql, &rows); err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, nil
	}
	return rows[0].Count, nil
}

func (s *Store) VariantReport(variantID int64) (Report, error) {
	sql := fmt.Sprintf(`
SELECT
	COALESCE(COUNT(*), 0) AS total_candidates,
	COALESCE(SUM(CASE WHEN c.error = '' THEN 1 ELSE 0 END), 0) AS generated_candidates,
	COALESCE(SUM(CASE WHEN c.error <> '' THEN 1 ELSE 0 END), 0) AS failed_candidates,
	COALESCE(SUM(CASE WHEN v.id IS NOT NULL THEN 1 ELSE 0 END), 0) AS judged_candidates,
	COALESCE(SUM(CASE WHEN v.winner = 'candidate' THEN 1 ELSE 0 END), 0) AS candidate_wins,
	COALESCE(SUM(CASE WHEN v.winner = 'baseline' THEN 1 ELSE 0 END), 0) AS baseline_wins,
	COALESCE(SUM(CASE WHEN v.winner = 'tie' THEN 1 ELSE 0 END), 0) AS ties,
	COALESCE(SUM(CASE WHEN v.winner = 'both_bad' THEN 1 ELSE 0 END), 0) AS both_bad,
	COALESCE(AVG(CASE WHEN c.error = '' THEN c.cost_usd END), 0) AS candidate_avg_cost,
	COALESCE(AVG(CASE WHEN c.error = '' THEN r.cost_usd END), 0) AS baseline_avg_cost,
	COALESCE(AVG(CASE WHEN c.error = '' THEN c.latency_ms END), 0) AS candidate_avg_latency,
	COALESCE(AVG(CASE WHEN c.error = '' THEN r.latency_ms END), 0) AS baseline_avg_latency
FROM eval_candidates c
JOIN runs r ON r.id = c.run_id
LEFT JOIN eval_votes v ON v.candidate_id = c.id
WHERE c.variant_id = %d;`, variantID)

	var rows []Report
	if err := s.query(sql, &rows); err != nil {
		return Report{}, err
	}
	if len(rows) == 0 {
		return Report{}, nil
	}
	return rows[0], nil
}

func (s *Store) PhraseReport(variantID int64, phrase string) (PhraseStats, error) {
	if phrase == "" {
		return PhraseStats{}, nil
	}

	sql := fmt.Sprintf(`
SELECT
	COALESCE(SUM(CASE WHEN INSTR(LOWER(r.ai_message), LOWER(%s)) > 0 THEN 1 ELSE 0 END), 0) AS baseline_hits,
	COALESCE(SUM(CASE WHEN INSTR(LOWER(c.message), LOWER(%s)) > 0 THEN 1 ELSE 0 END), 0) AS candidate_hits,
	COALESCE(COUNT(*), 0) AS compared
FROM eval_candidates c
JOIN runs r ON r.id = c.run_id
WHERE c.variant_id = %d
  AND c.error = '';`,
		sqlText(phrase),
		sqlText(phrase),
		variantID,
	)
	var rows []PhraseStats
	if err := s.query(sql, &rows); err != nil {
		return PhraseStats{}, err
	}
	if len(rows) == 0 {
		return PhraseStats{}, nil
	}
	return rows[0], nil
}

func (s *Store) exec(sql string) error {
	return runSQLite(s.path, false, sql, nil)
}

func (s *Store) query(sql string, target any) error {
	return runSQLite(s.path, true, sql, target)
}

func runSQLite(dbPath string, jsonOutput bool, sql string, target any) error {
	args := []string{}
	if jsonOutput {
		args = append(args, "-json")
	}
	args = append(args, dbPath)

	cmd := exec.Command("sqlite3", args...)
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	cmd.Stdin = strings.NewReader("PRAGMA foreign_keys = ON;\n" + sql + "\n")

	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errOut.String())
		if msg == "" {
			msg = strings.TrimSpace(out.String())
		}
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("sqlite3 failed: %s", msg)
	}

	if !jsonOutput || target == nil {
		return nil
	}

	raw := strings.TrimSpace(out.String())
	if raw == "" {
		raw = "[]"
	}
	if err := json.Unmarshal([]byte(raw), target); err != nil {
		return fmt.Errorf("failed to parse sqlite json: %w", err)
	}
	return nil
}

func sqlText(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

func sqlFloat(v float64) string {
	return strconv.FormatFloat(v, 'f', 8, 64)
}

func boolAsInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
