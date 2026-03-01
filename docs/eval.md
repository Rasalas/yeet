# yeet eval

`yeet eval` helps you validate whether a prompt/model variant is actually better on your real commit contexts.

The workflow is intentionally separate from normal commits:

1. `yeet` commit flow logs runs locally (SQLite) after successful AI commits.
2. `yeet eval generate` creates candidate outputs for a bounded subset.
3. `yeet eval judge` lets you blind-judge baseline vs candidate.
4. `yeet eval report` summarizes outcomes and costs.

## Data and Privacy

- Storage is local only.
- Database path: `~/.local/share/yeet/yeet.db`
- No background generation.
- No automatic upload.
- `sqlite3` must be available on `PATH`.

## Commands

### `yeet eval plan`

Estimates how many runs will be selected and the approximate cost before generation.

Important flags:

- `--sample`: max historical runs to scan (default `20`)
- `--batch-size`: max candidates to generate (default `5`)
- `--max-cost-usd`: generation budget cap (default `1.00`)
- `--provider`: override provider for candidate generation
- `--model`: override model (requires non-`auto` provider)
- `--prompt-file`: use alternate system prompt
- `--edited-only`: only runs where baseline output was edited
- `--contains`: only runs where baseline output contains phrase

### `yeet eval generate`

Creates a new eval variant and generates candidates for selected runs.

Behavior:

- Uses real stored run contexts (branch/status/recent commits/diff).
- Applies the chosen prompt/model/provider to generate candidate messages.
- Enforces selection bounds from `--sample`, `--batch-size`, `--max-cost-usd`.
- Stores variant + candidate rows for later judging/reporting.

### `yeet eval judge`

Interactive blind judging.

- Randomizes display order per item (`Option A`/`Option B`).
- Keys:
  - `a`: option A wins
  - `b`: option B wins
  - `t`: tie
  - `x`: both bad
  - `q`: stop
- Judging is resumable; rerun command to continue.

### `yeet eval report`

Shows per-variant summary:

- Candidate/baseline wins, ties, both-bad counts
- Candidate decisive win-rate
- Average candidate vs baseline cost
- Average candidate vs baseline latency
- Phrase-hit comparison when `--contains` was used

## Example

```sh
yeet eval plan --sample 30 --batch-size 8 --max-cost-usd 1.00 --contains "for improved user"
yeet eval generate --sample 30 --batch-size 8 --max-cost-usd 1.00 --contains "for improved user" --prompt-file ./prompt-v2.txt
yeet eval judge --variant 7
yeet eval report --variant 7
```
