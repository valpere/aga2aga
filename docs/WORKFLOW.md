# Development Workflow

## Pipeline Overview

```
Idea / bug / design doc
         ↓
    /backlog          → GitHub issue (RFC 2119 requirements, acceptance criteria)
         ↓
    tech-lead         → architecture gate (approve / reject / revise)
         ↓
    /ship             → parent session implements using go-tdd skill (TDD: failing test → impl → green)
         ↓ (parallel)
    ┌──────────────────────────────────────┐
    │  go-code-reviewer   static-analysis  │
    │  security-reviewer                   │
    └──────────────────────────────────────┘
         ↓ (fixes applied, all green)
    gh pr create
         ↓
    /fix-review       → 3 external model rounds + Claude Arbiter
         ↓
    auto-merge        → close GitHub issue
```

---

## Commands

### `/backlog <description or issue-number>`

Formalizes an idea or rough note into a ready-to-implement GitHub issue.

**Input:** Free-text description, or a link to an existing issue to harden.

**Process:**
1. Light codebase discovery (read, grep — no modifications)
2. Classify: Feature | Bug | Chore | Research
3. Verify against architecture constraints (Clean Architecture, Transport abstraction, immutable spec fields)
4. Decide whether to split (multiple independent components → separate issues)
5. Draft GitHub issue with RFC 2119 requirements (`MUST`, `MUST NOT`, `SHOULD`, `MAY`)
6. Run tech-lead agent for architecture pre-approval
7. Apply correct labels + milestone, open issue

**Output:** GitHub issue ready for `/ship`.

---

### `/ship [issue-number]`

Executes the full implementation pipeline on a ready issue.

If no issue number is given, picks the highest-priority open issue without `status: in-progress`.

**Process:**
1. Add `status: in-progress` label to the issue
2. Create feature branch: `feat/<number>-<slug>` or `fix/<number>-<slug>`
3. Implement using **go-tdd** skill (strict TDD — the parent session is the implementer)
4. Launch parallel review agents: go-code-reviewer, static-analysis, security-reviewer
5. Apply all findings, verify `go test ./... && go vet ./...` green
6. Create PR — title links to issue, body contains `Closes #N`
7. Run `/fix-review` (4-round external + Arbiter)
8. Auto-merge after Arbiter approval

---

### `/fix-review [pr-number]`

4-round AI code review. Can be run standalone or auto-triggered by `/ship`.

See [4-Round Review Process](#4-round-review-process) below.

---

### `/review-arch [file or pr-number]`

Invokes the tech-lead agent in isolation for architecture review only.
Useful before implementing a large change to validate the design.

---

## Agents

### `pm-issue-writer`

Translates requirements into actionable GitHub issues.

**Responsibilities:**
- RFC 2119-compliant acceptance criteria
- Issue splitting when scope contains multiple independent components
- Architecture pre-check (Clean Architecture, spec compliance, immutable field rules)
- Correct label/milestone assignment

**Triggers:** `/backlog` command

---

### `tech-lead`

Architecture gate. Must approve before code-generator runs.

**Responsibilities:**
- Enforce Clean Architecture: `pkg/` has no imports from `internal/` or `cmd/`
- Enforce Transport abstraction: no direct Redis/gossip calls from `pkg/document`, `pkg/protocol`
- Enforce protocol compliance: Skills Document spec (types, required fields, lifecycle transitions)
- Reject designs that mutate immutable genome fields or weaken `constraints.hard`
- Approve / reject / revise with specific corrected design example

**Model:** Opus (complex architectural reasoning)

---

### `go-code-reviewer`

Go code review agent. Reviews implementation quality, protocol compliance, and test coverage.

**Responsibilities:**
- Plan alignment: did the implementation match the issue acceptance criteria?
- Go code quality: nil safety, error wrapping, goroutine leaks, table-driven tests
- Architecture / SOLID: no `pkg/` → `internal/` imports, transport abstraction respected
- Protocol compliance: DO_NOT_TOUCH patterns, immutable genome fields, lifecycle transitions
- Documentation: exported identifiers have godoc, non-obvious logic is commented
- Test coverage: all error paths tested, lifecycle transitions covered (valid and invalid)

**Verdict:** PASS / PASS WITH SUGGESTIONS / NEEDS WORK

---

### `static-analysis`

Ensures `go vet ./...` and `golangci-lint run` pass with zero violations.

**Applies only safe cosmetic fixes:**
- Unused imports
- Unused variables
- `gofmt` formatting

**Never touches:**
- Logic changes to pass vet (those go to code-generator)
- DO_NOT_TOUCH patterns

---

### `security-reviewer`

Audits for security issues specific to a crypto-identity protocol library.

**Checks:**
- No hardcoded keys or secrets
- Ed25519 key material never logged or exposed in error messages
- YAML parsing: enforce document size limits (prevent memory exhaustion)
- Signature verification: reject documents with missing or invalid signatures when strict mode is on
- No unsafe use of `encoding/json` with user-controlled input that could cause allocation attacks
- Lifecycle: proposals that weaken `constraints.hard` must be rejected, never silently accepted

---

## 4-Round Review Process

Adapted from ClubTasker `daily-clean-spark/.claude/skills/fix-review/`.

### Round Configuration

| Round | Model | Provider | Diff scope | Focus |
|-------|-------|----------|------------|-------|
| 1 | `deepseek/deepseek-v3.2` | OpenRouter | Full PR diff | Architecture, layer violations, logic errors |
| 2 | `qwen/qwen3-coder-next` | OpenRouter | Delta (Round 1 fixes) | Nil safety, error handling, Go idioms |
| 3 | `mistralai/codestral-latest` | OpenRouter | Delta (Round 2 fixes) | Security, edge cases, protocol compliance |
| 4 | Claude (Arbiter) | — | Full PR diff | Confirm/Escalate/Dismiss/Defer + independent scan |

Fallback provider: Ollama (`deepseek-coder-v2:latest` → `qwen2.5-coder:32b` → `codestral:latest`).

### Code Review Pyramid (Go + Protocol)

| Layer | What to check | Severity | Style? |
|-------|---------------|----------|--------|
| 1 | Clean Architecture violations (`pkg` importing `internal`), Transport abstraction leaks, protocol spec violations (wrong field names, invalid types) | error | — |
| 2 | Nil pointer risks, unhandled errors, goroutine leaks, unclosed channels, race conditions, incorrect state transitions | error / warning | — |
| 3 | Missing or inadequate documentation on exported identifiers, unexplained non-obvious logic | warning / suggestion | — |
| 4 | Test coverage: missing table-driven cases, uncovered error paths, missing lifecycle transition tests | warning / suggestion | — |
| 5 | `gofmt`, blank lines, import order | **never flagged** — automated | — |

### Arbiter Logic

After rounds 1–3, Claude reviews all open findings plus runs an independent pass:

- **CONFIRM** — real issue, fix it
- **ESCALATE** — real issue, severity understated by external models, fix and upgrade
- **DISMISS** — false positive (conflicts with CLAUDE.md or DO_NOT_TOUCH), skip with reason recorded
- **DEFER** — real but out of scope for this PR, log only

Arbiter always runs, even if rounds 1–3 stopped early (loop detected).

**Auto-merge:** `gh pr merge <number> --auto --merge` after Arbiter commit.

### Loop Detection

If ≥ 80% of new file:line identifiers in a round match the previous round, stop and jump to Arbiter.

### Commit Format

```
fix(pr#N): address review comments — round 1
fix(pr#N): address review comments — round 2
fix(pr#N): address review comments — round 3
fix(pr#N): arbiter round — confirm, escalate, and independent findings
```

---

## Quality Gates

Every PR must pass before Arbiter triggers auto-merge:

| Gate | Command | Who enforces |
|------|---------|--------------|
| Tests pass | `go test ./...` | code-generator + each review round |
| Static analysis | `go vet ./...` && `golangci-lint run` | static-analysis agent |
| Security scan | security-reviewer findings resolved | security-reviewer |
| CI green | GitHub Actions | automated |
| Arbiter approval | no open CONFIRM/ESCALATE items | Claude Arbiter |

---

## DO_NOT_TOUCH Patterns

Patterns that must never be modified without explicit justification. Checked by all review agents.

| Pattern | Location | Why |
|---------|----------|-----|
| Lifecycle transition table | `pkg/document/lifecycle.go` | Protocol spec §16 — modifying breaks wire compatibility |
| Immutable genome fields list | `pkg/document/types_genome.go` | Spec §5.4 — these can never change in a live agent |
| `ValidTransition()` signature | `pkg/document/lifecycle.go` | Consumed by validator and CLI; changing breaks callers |
| JSON Schema `$defs` names | `pkg/document/schema.yaml` | Used for `oneOf` dispatch; renaming breaks validation |
| `ProtocolVersion = "v1"` | `pkg/protocol/types.go` | Wire protocol constant |
| `constraints.hard` / `identity` fields | `pkg/document/types_genome.go` | Spec §5.6 — proposals that weaken hard constraints MUST be rejected |
| `// DO_NOT_TOUCH` comment blocks | any file | Explicitly protected sections — checked by static-analysis and security-reviewer |

This list grows as the codebase matures. Add entries here when a pattern is established and must not drift.

---

## Scaling Path

### Current (Phase 0–1)
Sequential invocation. `/ship` runs agents one at a time.

### Phase 2
Parallel review batch: go-code-reviewer, static-analysis, security-reviewer launched as concurrent subagents.

### Phase 3+
Multi-issue parallelism via git worktrees. Multiple `/ship` invocations on separate issues simultaneously.
