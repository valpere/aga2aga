# /ship

Full implementation pipeline: issue → branch → TDD → review → PR → fix-review → auto-merge.

**Usage:** `/ship [issue-number]`

If no issue number: pick the highest-priority open issue with a `phase: 0 bootstrap` or `phase: 1 document` label and no `status: in-progress` label.

---

## Steps

### 1. Resolve Issue

```bash
gh issue view {number} --repo valpere/aga2aga --json number,title,body,labels,milestone
```

Add `status: in-progress` label:
```bash
gh issue edit {number} --repo valpere/aga2aga --add-label "status: in-progress"
```

### 2. Create Branch

```bash
# Feature / task:
git checkout -b feat/{number}-{kebab-slug}

# Bug fix:
git checkout -b fix/{number}-{kebab-slug}

# Chore:
git checkout -b chore/{number}-{kebab-slug}
```

Slug = issue title lowercased, spaces → hyphens, strip special chars, max 40 chars.

### 3. Plan (if non-trivial)

If the issue affects more than 1 component or has >3 acceptance criteria:
- Invoke `go-writing-plans` skill for task decomposition
- Save plan to `~/.claude/plans/` (Claude Code default location)

### 4. Implement (TDD)

For each task in the plan:
- Invoke `go-tdd` skill
- Write failing test first — verify RED (`FAIL` output, not compile error)
- Write minimal code — verify GREEN
- `go test ./... && go vet ./...` must pass after each task
- Commit after each GREEN + passing suite:
  ```bash
  git add {files}
  git commit -m "{type}({scope}): {description}"
  ```

### 5. Parallel Review

After all tasks complete, run in parallel:
- `go-code-reviewer` agent — plan alignment + Go idioms + protocol compliance
- `static-analysis` agent — `go vet` + `golangci-lint`, safe cosmetic fixes only
- `security-reviewer` agent — crypto/YAML/protocol threat model

Apply all findings that don't break tests. Commit:
```bash
git commit -m "review({scope}): apply parallel review findings"
```

### 6. Create PR

```bash
gh pr create \
  --repo valpere/aga2aga \
  --title "{type}: {issue title}" \
  --body "$(cat <<'EOF'
## Summary

- {bullet 1}
- {bullet 2}

## Test plan

- [ ] go test ./... passes
- [ ] go vet ./... clean
- [ ] golangci-lint run clean

Closes #{number}
EOF
)"
```

### 7. Fix-Review

```bash
# Invoke fix-review skill with the PR number
```

The skill runs 3 external model rounds + Claude Arbiter, then auto-merges.

### 8. Post-Merge Cleanup

The fix-review skill queues auto-merge and waits for the PR to reach MERGED state. Once fix-review confirms the merge, remove the label and return to main:

```bash
gh issue edit {number} --repo valpere/aga2aga --remove-label "status: in-progress"
git checkout main && git pull
```

If fix-review timed out waiting for merge, check `gh pr view {pr-number} --json state` before removing the label.

---

## Rules

- Never push directly to main — `main` requires a PR; use feature branches
- Never skip the TDD cycle — tests MUST fail before implementation
- Never skip parallel review — all three agents must run
- Never merge without fix-review — even if parallel review finds nothing
- **Never merge without green CI** — `gh pr checks {number}` must show all checks passing before merge; the repo ruleset does NOT require passing checks so auto-merge will complete even with failures
- Branch must have a corresponding open issue — no orphan branches
- `go test ./... && go vet ./...` must be green before creating the PR
