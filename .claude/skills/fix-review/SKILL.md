---
name: fix-review
description: "4-round AI code review for aga2aga PRs. Three external models review the diff; Claude acts as Arbiter in round 4 — confirms, dismisses, escalates, then auto-merges. Usage: /fix-review [PR-number]"
metadata:
  version: "1.0.0"
  domain: gateway
  triggers: fix-review, code review, pr review, review pr
  scope: review
---

# Skill: /fix-review
# aga2aga — Automated PR Review Comment Fixer

---

## OVERVIEW

```
Round 1: (provider round_1 model) — full PR diff  → fix → push
Round 2: (provider round_2 model) — delta diff    → fix → push
Round 3: (provider round_3 model) — delta diff    → fix → push
Round 4: Claude (Arbiter)         — full PR diff  → arbitrate → fix → push → auto-merge
Stop early if: no issues found | identical issues to previous round (≥ 80% overlap)
```

Rounds 1-3 call an external inference provider (OpenRouter or Ollama).
Round 4 is Claude itself — no API call. Claude reviews all prior findings, confirms what's real, dismisses noise, catches anything missed, then merges.

**Maximum 4 rounds. Never more. Arbiter always runs — cannot be skipped.**

---

## STEP 0: Resolve PR + Load Config

```bash
# If no argument, detect from current branch:
gh pr view --json number --jq '.number'
BASE_BRANCH=$(gh pr view {number} --json baseRefName --jq '.baseRefName')
```

If no open PR: `"No open PR found. Create a PR first or pass a number: /fix-review 42"`

Load `.claude/skills/fix-review/config.yaml` — extract `provider`, model names, `diff_scope`, `post_summary_to_pr`.

**Provider selection:** If `provider: ask`, present options and save choice with `sed -i`.
Load API key from environment (`OPENROUTER_API_KEY` or `OLLAMA_API_KEY`).

---

## STEP 1: Get the Diff

**Round 1** — full PR diff:
```bash
DIFF=$(git diff $(git merge-base HEAD origin/${BASE_BRANCH})...HEAD)
```

**Rounds 2-3** — delta (default) or full per `diff_scope`:
```bash
DIFF=$(git diff HEAD~1)                                           # delta
DIFF=$(git diff $(git merge-base HEAD origin/${BASE_BRANCH})...HEAD)  # full
```

If `DIFF` is empty for a delta round, fall back to the full PR diff.

---

## STEP 2: Call the Model

Use `curl` with the provider's API. Write the payload to a temp file to avoid shell-quoting issues with large diffs.

**Review prompt:**

```
You are a senior Go engineer reviewing code for the aga2aga MCP Gateway project.
Review the following git diff using the aga2aga Code Review Pyramid:

  5 (top) — Style          → DO NOT FLAG. gofmt + golangci-lint handle this automatically.
  4       — Tests          → Missing table-driven tests, uncovered error paths, fixtures not valid per spec.
  3       — Documentation  → Missing exported identifier docs, unexplained non-obvious logic.
  2       — Implementation → nil panics, unhandled errors (%w wrapping), goroutine leaks,
                             race conditions, incorrect lifecycle transitions, wrong field types.
  1 (base)— Architecture   → Clean Architecture violations (pkg/ importing internal/ or cmd/),
                             Transport abstraction leaks (Redis in pkg/), protocol spec violations
                             (wrong field names, invalid message types, DO_NOT_TOUCH violations).

Return ONLY a JSON array — no prose, no markdown fences, just raw JSON.
Each item must have exactly these fields:
  "file"     — relative file path (string)
  "line"     — line number on the + side of the diff (integer)
  "layer"    — pyramid layer 1–4 (integer)
  "severity" — "error" | "warning" | "suggestion" (string)
  "body"     — description of the issue and how to fix it (string)

Severity guide:
  error      — must fix before merge (bug, arch violation, DO_NOT_TOUCH breach)
  warning    — should fix (missing test coverage, undocumented exported API)
  suggestion — nice to have

DO NOT flag: formatting, import ordering, gofmt style (layer 5 — automated).
DO NOT flag code not present in this diff.
If no issues: return []

Git diff:
---
{DIFF}
---
```

Parse response as JSON. If parse fails, retry once with:
`"Your previous response was not valid JSON. Return ONLY the JSON array, no other text."`
If second failure: skip this reviewer and note in summary.

If zero items: `"No issues found by {model} in round {N}. Nothing to fix."` → proceed to Step 6.

---

## STEP 3: Snapshot Comment IDs

```
round_{N}_comment_ids = ["{file}:{line}", ...]
```

Used in Step 7 for loop detection.

---

## STEP 4: Group and Prioritise

Group by file path, then by proximity (within 10 lines = one cluster).

Sort: Layer 1 errors → Layer 1 warnings → Layer 2 errors → Layer 2 warnings → Layers 3-4 → suggestions.

---

## STEP 5: Fix Each Cluster

Read the file, understand full context, apply the fix with the Edit tool.

**Principles:**
- Minimal change to satisfy the reviewer's concern
- Do NOT refactor surrounding code
- Conservative interpretation for ambiguous comments
- If a comment conflicts with `CLAUDE.md` DO_NOT_TOUCH patterns → skip and surface to user

**After all fixes in the round:**
```bash
go test ./... && go vet ./...
```

If tests fail: identify the breaking fix, revert it with Edit, note as "skipped — caused test failure".

---

## STEP 6: Commit and Push

```bash
git add {files...}
git commit -m "fix(pr#{number}): address review comments — round {N}

{bullet list of what was fixed}

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
git push
```

If `post_summary_to_pr: true`:
```bash
gh pr comment {number} --body "<details>
<summary>Round {N} — {model} via {provider} ({count} issues found, {fixed} fixed)</summary>

{table: file:line | layer | severity | status | body}

</details>"
```

---

## STEP 7: Loop Detection

Before each new round, compare identifiers against the previous round.

**If ≥ 80% map to same file+line as previous round:** stop with warning listing affected locations. Still run Arbiter.

---

## STEP 8: Arbiter Round (Round 4) — Claude's Judgment

Get full current PR diff:
```bash
DIFF=$(git diff $(git merge-base HEAD origin/${BASE_BRANCH})...HEAD)
```

Compile findings log for rounds 1-3: model, every item flagged, status (fixed/skipped/open).

For each still-open item, rule:

| Ruling | Meaning | Action |
|--------|---------|--------|
| **CONFIRM** | Real issue, model was right | Fix now |
| **ESCALATE** | Real, more severe than flagged | Fix now, note upgrade |
| **DISMISS** | False positive / conflicts with project patterns | Skip, note reason |
| **DEFER** | Real but out of scope for this PR | Log, do not fix |

Perform **independent scan** of full diff — layers 1-4 only, never layer 5.

Apply all CONFIRM + ESCALATE fixes. Run tests:
```bash
go test ./... && go vet ./...
```
Revert any fix that breaks tests; note as "reverted — caused test failure".

```bash
git add {files...}
git commit -m "fix(pr#{number}): arbiter round — confirm, escalate, and independent findings

{bullet list: confirmed | escalated | new | dismissed}

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
git push
```

Skip commit entirely if zero fixes.

Post collapsible PR comment if enabled.

---

## STEP 9: Auto-merge

The `main` branch requires a PR (ruleset "prs"). Auto-merge is enabled on the repo, so `--auto` queues the merge and GitHub completes it once all required checks pass.

```bash
gh pr merge {number} --auto --merge
```

Resolve conflicts before this step — `--auto` does not handle them.

**Wait for merge to complete** (poll, 5-minute timeout):
```bash
for i in $(seq 1 60); do
  STATE=$(gh pr view {number} --repo valpere/aga2aga --json state --jq '.state')
  [ "$STATE" = "MERGED" ] && break
  echo "Waiting for merge... ($((i * 5))s)"
  sleep 5
done
STATE=$(gh pr view {number} --repo valpere/aga2aga --json state --jq '.state')
[ "$STATE" != "MERGED" ] && echo "WARNING: PR not yet merged after 5 minutes — check GitHub Actions"
```

After confirmed merge, pull main and print final summary table: fixed / escalated / dismissed / deferred / skipped.

```bash
git checkout main && git pull
```

---

## RULES

1. **Never force-push** — regular `git push` only
2. **Never push directly to main** — `main` requires a PR; all commits go on feature branches and merge via `gh pr merge`
3. **Never modify test files** to make tests pass — fix source code
4. **Never touch unrelated code** — only files in review comments or Arbiter findings
5. **4 rounds maximum** — hard stop
6. **DO_NOT_TOUCH conflicts** — skip and surface to user
7. **One commit per round** — batch all fixes
8. **Tests must pass before pushing** — revert breaking fix, continue
9. **JSON parse failure** — retry once, skip reviewer on second failure
10. **Provider switch is permanent** — `sed` into config.yaml immediately
11. **Arbiter always runs** — even if all 3 model rounds stop early
