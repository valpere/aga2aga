# Development Workflow

A practical guide to developing aga2aga from idea to merged code. Covers the full pipeline, every command, every agent, and how they fit together.

---

## Table of Contents

1. [Mental Model](#mental-model)
2. [Prerequisites](#prerequisites)
3. [Quick Reference](#quick-reference)
4. [The Full Pipeline](#the-full-pipeline)
5. [Commands](#commands)
   - [/backlog](#backlog)
   - [/ship](#ship)
   - [/fix-review](#fix-review)
   - [/review-arch](#review-arch)
6. [Agents](#agents)
7. [The 4-Round Review](#the-4-round-review)
8. [GitHub Issues and Labels](#github-issues-and-labels)
9. [Milestones](#milestones)
10. [Working with Skills](#working-with-skills)
11. [Quality Gates](#quality-gates)
12. [DO_NOT_TOUCH Patterns](#do_not_touch-patterns)
13. [Day-to-Day Patterns](#day-to-day-patterns)
14. [Troubleshooting](#troubleshooting)

---

## Mental Model

```
PLANNING     /backlog  →  GitHub issue (RFC 2119 requirements)
                ↓
GATEKEEPING  tech-lead →  architecture approval
                ↓
EXECUTION    /ship     →  TDD code → parallel review → PR → 4-round AI review → merge
```

**Everything starts with an issue.** No code is written without a well-formed GitHub issue that has passed the architecture gate.

**Everything ends with `/fix-review`.** No PR merges without three diverse external AI models and a Claude Arbiter pass.

---

## Prerequisites

```bash
# Required tools
go 1.24+          # go version
gh                # GitHub CLI — gh auth status
golangci-lint     # golangci-lint --version

# OpenRouter API key (for /fix-review external models)
# Add to .env.local:
OPENROUTER_API_KEY=sk-or-...

# Ollama (optional fallback for /fix-review)
# ollama pull deepseek-coder-v2:latest
# ollama pull qwen2.5-coder:32b
# ollama pull codestral:latest
```

Plugins (install once):

```
/plugin install superpowers@claude-plugins-official
/plugin install mcp-server-dev@claude-plugins-official
```

Verify with `/skills list`. You should see `superpowers:brainstorming`, `superpowers:writing-plans`, `superpowers:test-driven-development`, `superpowers:systematic-debugging`, `mcp-server-dev:build-mcp-server`.

---

## Quick Reference

| Goal | Command |
|------|---------|
| Turn an idea into an issue | `/backlog <description>` |
| Implement next priority issue | `/ship` |
| Implement a specific issue | `/ship 12` |
| Review an open PR | `/fix-review 14` |
| Check architecture before coding | `/review-arch` |
| Debug something broken | `Skill superpowers:systematic-debugging` |
| Run all tests | `go test ./...` |
| Run one test | `go test -run TestParseFrontMatter ./pkg/document/` |
| Format code | `gofmt -w .` |
| Static analysis | `go vet ./... && golangci-lint run` |

---

## The Full Pipeline

```
1.  You describe an idea to /backlog
        ↓
2.  pm-issue-writer agent explores the codebase lightly,
    classifies the change (Feature/Bug/Chore/Research),
    checks architecture constraints, decides whether to split,
    drafts a GitHub issue with MUST/SHOULD/MAY requirements
        ↓
3.  tech-lead agent reviews the design for architecture compliance
    APPROVE → issue is opened and labeled
    REJECT  → returned to you with specific corrected design
        ↓
4.  You (or /ship) pick the issue
        ↓
5.  The parent session (Claude) implements using the go-tdd skill:
    creates a branch, writes a failing test, implements minimum code
    to make it pass, refactors, repeats per task in the plan
        ↓
6.  Parallel review agents run (after implementation):
      go-code-reviewer  → plan alignment, Go quality, protocol compliance, tests
      static-analysis   → go vet + golangci-lint, cosmetic fixes only
      security-reviewer → crypto, YAML, protocol enforcement
        ↓
7.  All findings applied, go test ./... && go vet ./... green
        ↓
8.  PR created: "feat: <description> (closes #N)"
        ↓
9.  /fix-review runs:
      Round 1: DeepSeek V3.2   (full PR diff)
      Round 2: Qwen3-Coder     (delta diff — only round 1's changes)
      Round 3: Devstral        (delta diff — only round 2's changes)
      Round 4: Claude Arbiter  (full diff + prior findings → confirm/dismiss → auto-merge)
        ↓
10. CI passes, PR merges, issue closes automatically
```

Total elapsed time for a small-to-medium issue: 20–40 minutes.

---

## Commands

### /backlog

**Usage:** `/backlog <description>`

Takes a rough description and produces a GitHub issue with testable acceptance criteria, correct labels, and milestone assignment.

**Process:**
1. Light codebase discovery (read, grep — no modifications)
2. Classify: Feature | Bug | Chore | Research
3. Verify against architecture constraints (Clean Architecture, Transport abstraction, immutable spec fields)
4. Decide whether to split (multiple independent components → separate issues)
5. Draft requirements with RFC 2119 keywords — `MUST`, `MUST NOT`, `SHOULD`, `MAY`
6. Invoke tech-lead for architecture approval
7. Open the issue with labels and milestone

**Examples:**

```
/backlog add SplitFrontMatter function to pkg/document/parser.go
/backlog the validator is not catching missing 'from' field on task.request documents
/backlog phase 2: implement Redis Streams transport satisfying the Transport interface
```

**Output — a GitHub issue like:**

```markdown
## What & Why
The parser MUST split any input document at the `---` YAML front matter
delimiters before further processing.

## Acceptance Criteria
- [ ] `SplitFrontMatter([]byte) (yamlBytes []byte, body string, err error)` is exported
- [ ] Returns error when no opening `---` found
- [ ] Returns empty string body (not error) when body is absent
- [ ] go test ./... passes
- [ ] go vet ./... clean
```

**If tech-lead rejects the design:** Read the rejection — it contains the correct approach. Adjust your description and re-run `/backlog`.

---

### /ship

**Usage:** `/ship [issue-number]`

Runs the full implementation pipeline on a GitHub issue — branch creation, TDD implementation, parallel review, PR creation, and 4-round AI review.

**Examples:**

```
/ship        # picks highest-priority open issue automatically
/ship 12     # ships issue #12 specifically
```

**Automatic issue selection** (when no number given):

Picks the highest-priority open issue that has no `status: in-progress` label and has a `phase: 0 bootstrap` or `phase: 1 document` label (current milestone). Priority order: `critical` → `high` → `medium` → `low`.

**Branch naming:**

| Issue type | Branch format |
|------------|---------------|
| `type: feature` or `type: task` | `feat/<number>-<slug>` |
| `type: bug` | `fix/<number>-<slug>` |
| `type: chore` | `chore/<number>-<slug>` |
| `type: research` | `research/<number>-<slug>` |

**Step by step:**

```
1.  Adds label: status: in-progress
2.  git checkout -b feat/12-<slug>
3.  Invokes go-tdd skill — failing test → impl → green → commit (repeat per task)
4.  Runs in parallel:
      go-code-reviewer, static-analysis, security-reviewer
5.  Applies all findings
6.  go test ./... && go vet ./... — must be green before continuing
7.  gh pr create — body contains "Closes #12"
8.  /fix-review <pr-number>
9.  After Arbiter merge: removes status: in-progress label
```

**If tests fail after 3 fix attempts:** `/ship` surfaces the problem to you for architectural guidance before continuing. Three failures signals a design issue, not a bug.

**Intervention:** You can interrupt at any time. The state is always in git.

---

### /fix-review

**Usage:** `/fix-review [pr-number]`

Runs the 4-round AI code review on an open PR. Auto-triggered by `/ship`, but can also be run standalone.

**Examples:**

```
/fix-review           # reviews the PR for current branch
/fix-review 14        # reviews PR #14 specifically
```

**When to run it manually:**
- After making manual commits to a PR
- After a review round was interrupted
- On a PR created outside the `/ship` pipeline
- To re-review after addressing human reviewer comments

See [The 4-Round Review](#the-4-round-review) for full details.

---

### /review-arch

**Usage:** `/review-arch [file or pr-number]`

Invokes the tech-lead agent for an architecture review without starting implementation.

**Examples:**

```
/review-arch              # review current branch's uncommitted design
/review-arch 14           # review PR #14's architecture
/review-arch pkg/document # review the document package's current structure
```

**When to use it:**
- Before starting a large feature to validate the approach
- When uncertain whether a package boundary decision is correct
- After a tech-lead REJECT in `/backlog`, to iterate on the design

**Output:** APPROVE or REJECT with specific issues and corrected design examples.

---

## Agents

Agents are autonomous subprocesses orchestrated by the commands. Knowing what each one does helps you understand what's happening and when to intervene.

### pm-issue-writer

**Invoked by:** `/backlog`

Translates rough descriptions into well-formed GitHub issues. Never modifies files — read-only codebase access.

- Splits issues when the description contains multiple independent components
- Rejects scope that violates architecture (e.g., "add Redis call to pkg/document")
- Uses RFC 2119 keywords — acceptance criteria are testable statements, not vague goals

---

### tech-lead

**Invoked by:** `/backlog` (automatically), `/review-arch` (directly). Uses Opus model for complex architectural reasoning.

Architecture gate. Must approve before any code is written.

- `pkg/` packages must not import `internal/` or `cmd/`
- Transport abstraction: no Redis/gossip calls from `pkg/document` or `pkg/protocol`
- Protocol spec compliance: field names, required fields, lifecycle transitions
- Immutable genome fields must not be modified in any mutation path

**Output:** APPROVE (with advisory notes if any) or REJECT (with specific issue + corrected design).

---

### go-code-reviewer

**Invoked by:** `/ship` (after implementation, in parallel with static-analysis and security-reviewer)

Reviews implementation quality, protocol compliance, and test coverage.

1. Plan alignment — did implementation match all acceptance criteria?
2. Go code quality — nil safety, error wrapping with `%w`, goroutine safety, idiomatic patterns
3. Architecture / SOLID — `pkg/` never imports `internal/` or `cmd/`
4. Protocol compliance — DO_NOT_TOUCH patterns respected; lifecycle transitions valid
5. Documentation — exported identifiers have godoc; non-obvious logic explained
6. Tests — table-driven with named cases; error paths covered; lifecycle transitions tested

**Verdict:** PASS / PASS WITH SUGGESTIONS / NEEDS WORK. NEEDS WORK blocks `/ship`.

---

### static-analysis

**Invoked by:** `/ship` (after implementation, in parallel)

Makes `go vet ./...` and `golangci-lint run` pass. Applies only safe cosmetic fixes (unused imports, unused variables, `gofmt` formatting). Never touches logic or DO_NOT_TOUCH patterns.

---

### security-reviewer

**Invoked by:** `/ship` (after implementation, in parallel)

Audits for security issues specific to a crypto-identity protocol library.

| Area | What it looks for |
|------|------------------|
| Crypto | Ed25519 key material in logs or error strings; `math/rand` used for security; hardcoded keys |
| YAML parsing | Documents parsed without size-limit rejection; arbitrary key injection into structs |
| Protocol enforcement | Lifecycle bypass; `constraints.hard` weakening silently accepted; `from` field used for auth |
| General | Hardcoded secrets; missing error checks on security-relevant operations |

**Output:** Severity-ranked findings (CRITICAL / HIGH / MEDIUM / LOW). CRITICAL findings block `/ship`.

---

## The 4-Round Review

Every PR goes through this before merging. `/fix-review` orchestrates it.

### Round Configuration

| Round | Model | Provider | Diff scope |
|-------|-------|----------|------------|
| 1 | `deepseek/deepseek-v3.2-20251201` | OpenRouter | Full PR diff |
| 2 | `qwen/qwen3-coder-next` | OpenRouter | Delta (Round 1 fixes) |
| 3 | `mistralai/devstral-2512` | OpenRouter | Delta (Round 2 fixes) |
| 4 | Claude (Arbiter) | — | Full PR diff + prior findings |

Fallback provider: Ollama. Switch in `.claude/skills/fix-review/config.yaml`:

```yaml
provider: ollama
```

### Why three different external models?

Each model has different training data and different blind spots. Using three different model families (DeepSeek / Alibaba / Mistral) eliminates the "blurred eye" effect. Delta diffs in rounds 2 and 3 keep token cost low.

### Code Review Pyramid

All three external models and the Arbiter use this pyramid. Layer 1 is most important; Layer 5 is never reviewed (automated).

| Layer | What to check | Severity |
|-------|---------------|----------|
| 1 | Clean Architecture violations (`pkg/` importing `internal/`), Transport abstraction leaks, protocol spec violations | error |
| 2 | Nil pointer risks, unhandled errors (`%w` wrapping), goroutine leaks, race conditions, incorrect state transitions | error / warning |
| 3 | Missing godoc on exported identifiers, unexplained non-obvious logic | warning / suggestion |
| 4 | Missing table-driven test cases, uncovered error paths, lifecycle transitions not tested | warning / suggestion |
| 5 | `gofmt`, blank lines, import order | **never flagged** — automated |

### Arbiter Logic (Round 4)

After rounds 1–3, Claude reviews all open findings plus runs an independent scan of the full diff:

| Ruling | Meaning | Action |
|--------|---------|--------|
| CONFIRM | Real issue, model was right | Fix it |
| ESCALATE | Real, severity understated | Fix it, upgrade severity |
| DISMISS | False positive, conflicts with CLAUDE.md or DO_NOT_TOUCH | Skip, record reason |
| DEFER | Real but out of scope for this PR | Log only, do not fix |

Arbiter always runs, even if rounds 1–3 stopped early (loop detected).

**Auto-merge:** `gh pr merge <number> --auto --merge` after Arbiter commit. CI must pass before merge completes.

### Loop Detection

If ≥ 80% of new file:line identifiers in a round match the previous round, stop and jump to Arbiter.

### Commit Format

```
fix(pr#N): address review comments — round 1
fix(pr#N): address review comments — round 2
fix(pr#N): address review comments — round 3
fix(pr#N): arbiter round — confirm, escalate, and independent findings
```

### Reading the PR comments

After each round, a collapsible comment is posted:

```
Round N — <model> via <provider> (M issues found, K fixed)

| file:line | layer | severity | status | body |
```

Common DISMISS reasons from the Arbiter:
- "Conflicts with DO_NOT_TOUCH: ValidTransition() signature"
- "False positive: error is intentionally not wrapped here (sentinel error)"
- "Conflicts with CLAUDE.md: interface defined at consumer side, not provider"

---

## GitHub Issues and Labels

### Label System

Every issue should have one label from each relevant group.

**Type:**

| Label | Use for |
|-------|---------|
| `type: feature` | New capability |
| `type: bug` | Something broken |
| `type: task` | Implementation work item |
| `type: chore` | CI, config, tooling |
| `type: docs` | Documentation only |
| `type: test` | Tests only |
| `type: research` | Investigation / spike |

**Priority:**

| Label | Meaning |
|-------|---------|
| `priority: critical` | Blocker — drop everything |
| `priority: high` | Current sprint |
| `priority: medium` | Normal |
| `priority: low` | Nice to have |

**Phase:**

| Label | Phase |
|-------|-------|
| `phase: 0 bootstrap` | Go module, CI, Dockerfile, tooling |
| `phase: 1 document` | envelope document engine |
| `phase: 2 gateway` | MCP Gateway + Redis transport |
| `phase: 3 identity` | Ed25519 identity + trust graph |
| `phase: 4 negotiation` | Negotiation protocol |
| `phase: 5 p2p` | Gossip P2P transport |

**Component:** `component: document`, `component: protocol`, `component: transport`, `component: identity`, `component: negotiation`, `component: gateway`, `component: cli`, `component: ci`

**Status** (set by workflow, not manually):

| Label | Set when |
|-------|---------|
| `status: in-progress` | `/ship` starts working on the issue |
| `status: blocked` | Cannot proceed — add a comment explaining why |
| `status: needs-review` | PR open, waiting for `/fix-review` |
| `status: stale` | No activity for 30+ days |

### Issue Hygiene

- One concern per issue. "And also…" means two issues.
- Acceptance criteria must be testable. "Code should be clean" is not a criterion. "go vet ./... passes" is.
- Close issues via the PR body (`Closes #N`), not manually.
- If blocked, add `status: blocked` and a comment explaining why.

---

## Milestones

| Milestone | Due | Gate criteria |
|-----------|-----|---------------|
| M1: First Document | Apr 6 2026 | Parse, validate, create all 24 message types. 100+ tests green. |
| M2: First Task | Apr 27 2026 | Claude Code submits task via MCP, receives result from another agent. |
| M3: Trusted Agents | May 18 2026 | All messages signed. Trust scores affect routing. |
| M4: First Negotiation | Jun 8 2026 | Two agents negotiate and reach agreement autonomously. |
| M5: No Redis | Jul 20 2026 | Agents communicate P2P without central infrastructure. |
| M6: First Spawn | Sep 14 2026 | New agent spawned from genome, passes sandbox, promoted to candidate. |

---

## Working with Skills

Skills are invoked with the `Skill` tool, not by reading skill files.

| Skill | Use when |
|-------|---------|
| `superpowers:brainstorming` | Before implementing anything non-trivial |
| `superpowers:writing-plans` | After brainstorming — decompose into TDD tasks |
| `superpowers:test-driven-development` | Any time you're writing Go code |
| `superpowers:systematic-debugging` | A bug has resisted 2+ fix attempts |
| `mcp-server-dev:build-mcp-server` | Phase 2 — building the MCP Gateway |
| `aga2aga-protocol` | Working with envelope documents, message types, lifecycle |
| `go-tdd` | Go-specific TDD conventions |
| `go-code-reviewer` | Reviewing Go code with protocol compliance |

**The 1% rule:** If there is even a 1% chance a skill is relevant, invoke it. The cost of loading a skill is low; the cost of working without it is high.

---

## Quality Gates

Every PR must clear all gates before the Arbiter triggers auto-merge:

| Gate | Command | Enforced by |
|------|---------|-------------|
| Tests pass | `go test ./...` | code-generator + each review round |
| No vet violations | `go vet ./...` | static-analysis agent |
| Lint clean | `golangci-lint run` | static-analysis agent |
| Security scan clear | — | security-reviewer agent |
| CI green | GitHub Actions | automated |
| Arbiter: no open CONFIRM/ESCALATE | — | Claude Arbiter |

Run all gates locally:

```bash
go test ./... && go vet ./... && golangci-lint run
```

---

## DO_NOT_TOUCH Patterns

Patterns that must never be modified without explicit justification. Checked by all review agents.

| Pattern | Location | Why |
|---------|----------|-----|
| Lifecycle transition table | `pkg/document/lifecycle.go` | Protocol spec §16 — modifying breaks wire compatibility with live agents |
| Immutable genome fields list | `pkg/document/types_genome.go` | Spec §5.4 — these fields must survive agent mutation |
| `ValidTransition()` signature | `pkg/document/lifecycle.go` | Consumed by validator and CLI; signature change breaks callers |
| JSON Schema `$defs` names | `pkg/document/schema.yaml` | Used for `oneOf` dispatch; renaming silently breaks validation |
| `ProtocolVersion = "v1"` | `pkg/protocol/types.go` | Wire protocol constant — bump requires explicit migration plan |
| `constraints.hard` / `identity` fields | `pkg/document/types_genome.go` | Spec §5.6 — proposals that weaken hard constraints MUST be rejected |
| `// DO_NOT_TOUCH` comment blocks | any file | Explicitly protected sections — checked by all review agents |

**Adding a new DO_NOT_TOUCH pattern:**

1. Add a row to the table above
2. Add a comment in the source file: `// DO_NOT_TOUCH: <reason> — see docs/WORKFLOW.md`

---

## Day-to-Day Patterns

### Starting a new session

The session-start hook injects context automatically: current phase, open in-progress issues, key constraints.

### Starting a new feature

```
/backlog I want to add X so that Y
```

Review the issue that gets created. If the requirements look right, `/ship` it.

### Implementing from the backlog

```
/ship         # auto-select
/ship 12      # specific issue
```

Leave it running. Check back when done or if it surfaces a question.

### Handling a tech-lead rejection

The rejection message contains the correct approach:

> REJECT: `pkg/document/parser.go` importing `internal/gateway` violates Clean Architecture.
> Corrected approach: move the shared type to `pkg/protocol/types.go`.

Read it, adjust the description, re-run `/backlog` with the revised scope.

### Handling a failed test during /ship

If `/ship` stops after 3 failed fix attempts:

> Probable architectural issue: the `As[T]` generic function cannot be used with the current YAML inline marshaling strategy. Consider switching to a two-pass parse.

This is a design decision. Read the diagnosis, decide the approach, then:

```
/ship 12      # re-run after addressing the diagnosis
```

### Making a change directly (without /ship)

```bash
git checkout -b feat/12-my-change
# write code following TDD
gh pr create
/fix-review
```

### Debugging something

```
Skill superpowers:systematic-debugging
```

The skill walks through root cause investigation before proposing fixes. Three failed fix attempts without a root cause is a bad outcome — stop and use this skill.

---

## Troubleshooting

### `/fix-review` says "provider: ask" on first run

The config hasn't been set. The skill will prompt you once. Your choice is saved to `.claude/skills/fix-review/config.yaml`.

### OpenRouter returns 401

Check `.env.local`:

```bash
cat .env.local | grep OPENROUTER
```

If missing, add `OPENROUTER_API_KEY=sk-or-...` and re-run.

### A round keeps triggering loop detection

Loop detection fires when ≥ 80% of the same file:line pairs appear in two consecutive rounds. Usually means:
- The external model is flagging a DO_NOT_TOUCH pattern as a bug
- A previous fix introduced a regression that the next round also flags

The Arbiter will DISMISS false positives. If it's a real issue, the Arbiter will CONFIRM and fix it.

### go test fails after /fix-review commits

The Arbiter should have reverted any fix that broke tests. If it didn't:

```bash
go test ./...
# find the failing test, then:
git log --oneline
git revert <commit-hash>
/fix-review   # re-run so the Arbiter handles it differently
```

### /ship picks the wrong issue

Check the labels. `/ship` picks based on phase labels matching the current milestone and priority ordering. Add `status: blocked` to the wrong issue and re-run.

### tech-lead keeps rejecting the same design

Read the rejection message — it contains a specific corrected approach. If the same issue recurs, run `/review-arch` with the full package in scope for a broader architectural review before proceeding.

### Skill not found

```
/plugin install superpowers@claude-plugins-official
```
