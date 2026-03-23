# Developer Workflow User Guide

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

The workflow has three distinct layers:

```
PLANNING     /backlog  →  GitHub issue (RFC 2119 requirements)
                ↓
GATEKEEPING  tech-lead →  architecture approval
                ↓
EXECUTION    /ship     →  TDD code → parallel review → PR → 4-round AI review → merge
```

**Everything starts with an issue.**
No code is written without a well-formed GitHub issue that has passed the architecture gate.
This prevents wasted implementation work and keeps the history clean.

**Everything ends with `/fix-review`.**
No PR merges without going through three diverse external AI models and a Claude Arbiter pass.
This catches issues that slip through the author's blind spots.

---

## Prerequisites

**Environment:**

```bash
# Required tools
go 1.24+          # go version
gh                # GitHub CLI — gh auth status
golangci-lint     # golangci-lint --version
docker            # docker --version

# OpenRouter API key (for /fix-review external models)
# Add to .env.local:
OPENROUTER_API_KEY=sk-or-...

# Ollama (optional fallback for /fix-review)
# ollama pull deepseek-coder-v2:latest
# ollama pull qwen2.5-coder:32b
# ollama pull codestral:latest
```

**Plugins (install once):**

```
/plugin install superpowers@claude-plugins-official
/plugin install mcp-server-dev@claude-plugins-official
```

Verify:

```
/skills list
```

You should see `superpowers:brainstorming`, `superpowers:writing-plans`, `superpowers:test-driven-development`, `superpowers:systematic-debugging`, `mcp-server-dev:build-mcp-server` in the list.

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

From idea to merged PR:

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
      Round 1: DeepSeek v3.2   (full diff)
      Round 2: Qwen3-Coder     (delta diff — only round 1's changes)
      Round 3: Codestral       (delta diff — only round 2's changes)
      Round 4: Claude Arbiter  (full diff + prior findings)
        ↓
10. Arbiter commits final fixes, triggers gh pr merge --auto
        ↓
11. CI passes, PR merges, issue closes automatically
```

Total elapsed time for a small-to-medium issue: 20–40 minutes.

---

## Commands

### /backlog

**Usage:** `/backlog <description>`

**What it does:** Takes a rough description and produces a GitHub issue with testable acceptance criteria, correct labels, and milestone assignment.

**Examples:**

```
/backlog add SplitFrontMatter function to pkg/document/parser.go
```

```
/backlog the validator is not catching missing 'from' field on task.request documents
```

```
/backlog phase 2: implement Redis Streams transport satisfying the Transport interface
```

**What happens:**

1. pm-issue-writer reads relevant files (never modifies anything)
2. Classifies your request: Feature / Bug / Chore / Research
3. Checks against architecture constraints:
   - Does this touch the right package?
   - Does it respect the transport abstraction boundary?
   - Does it touch any DO_NOT_TOUCH patterns?
4. Decides whether to split (if you described two independent things, it opens two issues)
5. Writes requirements with RFC 2119 keywords — `MUST`, `MUST NOT`, `SHOULD`, `MAY`
6. Invokes tech-lead for architecture approval
7. Opens the issue with labels and milestone

**Output — a GitHub issue like:**

```markdown
## What & Why
The parser MUST split any input document at the `---` YAML front matter
delimiters before further processing. Required by the document engine as
the foundational parsing step.

## Acceptance Criteria
- [ ] `SplitFrontMatter([]byte) (yamlBytes []byte, body string, err error)`
      is exported from `pkg/document/`
- [ ] Returns error when no opening `---` found
- [ ] Returns empty string body (not error) when body is absent
- [ ] Round-trip: Serialize(Parse(raw)) equals raw for all test fixtures
- [ ] go test ./... passes
- [ ] go vet ./... clean

## Implementation Notes
- Edge case: `---` appearing inside the Markdown body must not be treated
  as a delimiter
- See tests/testdata/ for fixture files

## Dependencies
Unblocks: #N (validator), #N (builder)
```

**When to use it instead of the GitHub issue form directly:**

Use `/backlog` when you have a rough idea and want the architecture check and RFC 2119 hardening done automatically. Use the GitHub issue form directly when the requirements are already clear and you just need the tracking entry.

**If tech-lead rejects the design:**

The agent will return a specific objection with a corrected approach. Read it, adjust your description, and re-run `/backlog` with the revised framing.

---

### /ship

**Usage:** `/ship [issue-number]`

**What it does:** Runs the full implementation pipeline on a GitHub issue — from branch creation through TDD implementation, review passes, PR creation, and the 4-round AI review.

**Examples:**

```
/ship        # picks highest-priority open issue automatically
/ship 12     # ships issue #12 specifically
```

**Automatic issue selection** (when no number given):

Picks the highest-priority open issue that:
- Has no `status: in-progress` label
- Has a `phase: 0 bootstrap` or `phase: 1 document` label (current milestone)
- Is not marked `status: blocked`

Priority order: `priority: critical` → `priority: high` → `priority: medium` → `priority: low`

**Branch naming:**

| Issue type | Branch format |
|------------|---------------|
| `type: feature` or `type: task` | `feat/<number>-<slug>` |
| `type: bug` | `fix/<number>-<slug>` |
| `type: chore` | `chore/<number>-<slug>` |
| `type: research` | `research/<number>-<slug>` |

Example: issue #12 "add SplitFrontMatter function" → `feat/12-add-split-front-matter`

**What happens step by step:**

```
1.  Adds label: status: in-progress
2.  git checkout -b feat/12-<slug>
3.  Invokes superpowers:brainstorming
       (if issue touches >1 component — skipped for trivial changes)
4.  Invokes superpowers:writing-plans
       (decomposes into 2-5 minute TDD tasks with exact file paths)
5.  Invokes go-tdd skill (parent session implements directly)
       (implements each task: failing test → green → refactor → commit)
6.  Runs in parallel:
       go-code-reviewer  → plan alignment, Go quality, protocol compliance, tests
       static-analysis   → go vet + golangci-lint, cosmetic fixes
       security-reviewer → crypto/YAML/protocol audit
7.  Applies all findings
8.  go test ./... && go vet ./...  — must be green before continuing
9.  gh pr create --title "..." --body "...\n\nCloses #12"
10. /fix-review <pr-number>
11. After Arbiter merge: removes status: in-progress label
```

**If tests fail during step 8:**

The pipeline stops. The code-generator is invoked to fix the failing tests. If it fails 3 times, `/ship` surfaces the problem to you for architectural guidance before continuing.

**Intervention points:**

You can interrupt `/ship` at any time. The state is always in git — the branch exists with whatever commits were made, and you can inspect or modify before continuing.

---

### /fix-review

**Usage:** `/fix-review [pr-number]`

**What it does:** Runs the 4-round AI code review on an open PR. Auto-triggered by `/ship`, but can also be run standalone on any PR.

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

**The four rounds:**

```
Round 1 — DeepSeek v3.2 (OpenRouter)
  Input:  Full PR diff (merge-base...HEAD)
  Focus:  Architecture violations, Clean Architecture, protocol spec
  Output: JSON array of comments {file, line, layer, severity, body}
  Action: Fix all findings → commit → push

Round 2 — Qwen3-Coder Next (OpenRouter)
  Input:  Delta diff (only Round 1's changes)
  Focus:  Nil safety, error handling, Go idioms
  Action: Fix all findings → commit → push

Round 3 — Codestral (OpenRouter)
  Input:  Delta diff (only Round 2's changes)
  Focus:  Security, edge cases, goroutine safety, protocol compliance
  Action: Fix all findings → commit → push

Round 4 — Claude Arbiter
  Input:  Full PR diff + all prior findings with status (fixed/open)
  Action: CONFIRM/ESCALATE/DISMISS/DEFER each finding
           + independent pass on full diff
           → fix all CONFIRM + ESCALATE → commit → push
           → gh pr merge --auto --merge
```

**What each round commit looks like:**

```
fix(pr#14): address review comments — round 1
fix(pr#14): address review comments — round 2
fix(pr#14): address review comments — round 3
fix(pr#14): arbiter round — confirm, escalate, and independent findings
```

**What CONFIRM/ESCALATE/DISMISS/DEFER mean:**

| Ruling | Meaning | Action |
|--------|---------|--------|
| CONFIRM | Finding is real and correct | Fix it |
| ESCALATE | Real, but severity was understated | Fix it, upgrade severity in summary |
| DISMISS | False positive, contradicts CLAUDE.md or DO_NOT_TOUCH | Skip, record reason |
| DEFER | Real but out of scope for this PR | Log only, open follow-up issue |

**If a fix breaks tests:**

The fix is reverted automatically. The issue is noted in the round summary. The Arbiter will see it as an unresolved finding and decide whether to CONFIRM (try again with a different approach) or DEFER.

**If the external model returns invalid JSON:**

One retry. If it fails again, that round is skipped and noted in the summary. The Arbiter always runs regardless.

**Loop detection:**

If Round 2 or 3 finds ≥ 80% of the same file:line issues as the previous round, the pipeline detects a fix loop and skips directly to the Arbiter. This prevents infinite back-and-forth on the same lines.

**Provider fallback:**

If OpenRouter is unavailable or the API key is missing, `/fix-review` falls back to local Ollama models. Set the provider in `.claude/skills/fix-review/config.yaml`:

```yaml
provider: ollama   # switch from openrouter
```

**After merge:**

The PR is squash-merged into main. CI runs. The `Closes #N` line in the PR body automatically closes the linked issue.

---

### /review-arch

**Usage:** `/review-arch [file or pr-number]`

**What it does:** Invokes the tech-lead agent for an architecture review in isolation — without starting implementation.

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
- When refactoring package structure

**Output:** APPROVE or REJECT with specific issues and corrected design examples.

---

## Agents

Agents are autonomous subprocesses invoked by commands. You don't invoke them directly in normal workflow — the commands orchestrate them. But knowing what each one does helps you understand what's happening and when to intervene.

### pm-issue-writer

**Invoked by:** `/backlog`

**What it does:** Translates rough descriptions into well-formed GitHub issues.

**Never modifies files.** Read-only codebase access.

**Key behaviors:**
- Splits issues when the description contains multiple independent components
- Rejects scope that violates architecture (e.g., "add Redis call to pkg/document")
- Uses RFC 2119 keywords — acceptance criteria are testable statements, not vague goals

---

### tech-lead

**Invoked by:** `/backlog` (automatically), `/review-arch` (directly)

**What it does:** Architecture gate. Must approve before any code is written.

**Uses Opus model** for complex architectural reasoning.

**Checks:**
- `pkg/` packages don't import `internal/` or `cmd/`
- Transport abstraction: no Redis/gossip calls from `pkg/document` or `pkg/protocol`
- Protocol spec compliance: field names, required fields, lifecycle transitions
- Immutable genome fields not modified in any mutation path
- Change fits the current phase milestone

**Output:** APPROVE (with advisory notes if any) or REJECT (with specific issue + corrected design).

**If it rejects your idea:** Don't work around it. Read the rejection — it contains the correct approach. Revise and re-run `/backlog`.

---

### go-code-reviewer

**Invoked by:** `/ship` (after implementation, in parallel with static-analysis and security-reviewer)

**What it does:** Reviews implementation quality, protocol compliance, and test coverage.

**Review sections:**
1. Plan alignment — did implementation match all acceptance criteria?
2. Go code quality — nil safety, error wrapping with `%w`, goroutine safety, idiomatic patterns
3. Architecture / SOLID — `pkg/` never imports `internal/` or `cmd/`; interfaces at consumer side
4. Protocol compliance — DO_NOT_TOUCH patterns respected; lifecycle transitions valid; genome fields immutable
5. Documentation — exported identifiers have godoc; non-obvious logic explained
6. Tests — table-driven with named cases; error paths covered; all lifecycle transitions tested (valid and invalid); no live infrastructure required

**Verdict:** PASS / PASS WITH SUGGESTIONS / NEEDS WORK

NEEDS WORK blocks `/ship` until findings are resolved.

---

### static-analysis

**Invoked by:** `/ship` (after implementation, in parallel with go-code-reviewer)

**What it does:** Makes `go vet` and `golangci-lint` pass.

**Safe to fix automatically:**
- Unused imports
- Unused variables
- `gofmt` formatting violations

**Never touches:**
- Logic (even if it causes a lint warning — that goes to code-generator)
- DO_NOT_TOUCH patterns (see [below](#do_not_touch-patterns))

**After its fixes:** Runs `go vet ./... && golangci-lint run` to confirm zero violations.

---

### security-reviewer

**Invoked by:** `/ship` (after implementation, in parallel with go-code-reviewer)

**What it does:** Audits for security issues specific to a crypto-identity protocol library.

**Focus areas:**

| Area | What it looks for |
|------|------------------|
| Crypto | Ed25519 key material in logs or error strings; `math/rand` used for security; hardcoded keys or seeds |
| YAML parsing | Documents larger than the size limit parsed without rejection; arbitrary key injection into structs |
| Protocol enforcement | Lifecycle bypass (evaluation skipped); `constraints.hard` weakening accepted silently; `from` field accepted without verification in strict mode |
| General | Hardcoded secrets; `fmt.Sprintf` with user input in security paths; missing error checks on security-relevant operations |

**Output:** Severity-ranked findings (CRITICAL / HIGH / MEDIUM / LOW) with file:line, vulnerability type, and recommendation.

**CRITICAL findings** block `/ship` until resolved.

---

### go-code-reviewer

See the expanded section above — this entry is the same agent, listed here for reference in the agent index.

---

## The 4-Round Review

The most important quality gate. Every PR goes through this before merging.

### Why three different external models?

Each model has different training data, different architectural instincts, and different blind spots. Using three different model families (DeepSeek / Alibaba / Mistral) in sequence eliminates the "blurred eye" effect — patterns one model misses, another catches. Delta diffs in rounds 2 and 3 keep token cost low: only the changes from the previous round are reviewed, not the entire PR again.

### The Code Review Pyramid

All three external models and the Arbiter use this pyramid. Layer 1 is most important; Layer 5 is never reviewed (it's automated).

```
Layer 1 — Architecture             (highest priority)
  · pkg/ imports internal/ or cmd/
  · Transport abstraction violated (Redis call from pkg/document)
  · Protocol spec violation (wrong field name, invalid type)
  · Lifecycle transition not in spec §16

Layer 2 — Implementation bugs
  · Nil pointer dereference risk
  · Error return ignored
  · Goroutine leak (no exit condition)
  · Channel never closed
  · Race condition on shared state
  · Incorrect state machine transition

Layer 3 — Documentation
  · Exported identifier missing godoc comment
  · Non-obvious logic has no explanation
  · Complex algorithm undocumented

Layer 4 — Tests
  · Missing table-driven cases for error paths
  · Lifecycle state machine tests incomplete
  · New conditional branch has no test

Layer 5 — Style                    (never reviewed — gofmt handles it)
  · Formatting
  · Blank lines
  · Import order
```

**Findings outside Layer 1–4 are discarded.** The Arbiter will DISMISS any Layer 5 comment from an external model.

### Reading the PR comments

After each round, a collapsible comment is posted to the PR. It looks like:

```
## Round 1 Review — DeepSeek v3.2

| File | Line | Layer | Severity | Status |
|------|------|-------|----------|--------|
| pkg/document/parser.go | 42 | L1 | error | ✅ fixed |
| pkg/document/types.go | 17 | L2 | warning | ✅ fixed |

2 issues found, 2 fixed.
```

After the Arbiter:

```
## Arbiter Round

### Prior Findings
| ID | Ruling | Reason |
|----|--------|--------|
| parser.go:42 | CONFIRM | fixed in round 1 |
| types.go:17 | CONFIRM | fixed in round 1 |

### Independent Findings
| File | Line | Severity | Description |
|------|------|----------|-------------|
| pkg/document/parser.go | 88 | L2/warning | error return from yaml.Unmarshal not wrapped with %w |

1 independent finding fixed. Auto-merging.
```

### When the Arbiter DISMISS es something

This means the external model flagged something that conflicts with the project's established patterns (CLAUDE.md or DO_NOT_TOUCH). The reason is recorded in the Arbiter comment so you can read it. Common DISMISS reasons:

- "Conflicts with DO_NOT_TOUCH: ValidTransition() signature"
- "Conflicts with CLAUDE.md: interface defined at consumer side, not provider"
- "False positive: error is intentionally not wrapped here (sentinel error)"

---

## GitHub Issues and Labels

### Label System

Issues use a structured label taxonomy. Every issue should have one label from each relevant group.

**Type** — what kind of work is it?

| Label | Use for |
|-------|---------|
| `type: feature` | New capability |
| `type: bug` | Something broken |
| `type: task` | Implementation work item |
| `type: chore` | CI, config, tooling |
| `type: docs` | Documentation only |
| `type: test` | Tests only |
| `type: research` | Investigation / spike |

**Priority** — when should it be done?

| Label | Meaning |
|-------|---------|
| `priority: critical` | Blocker — drop everything |
| `priority: high` | Current sprint |
| `priority: medium` | Normal |
| `priority: low` | Nice to have |

**Phase** — which implementation phase does it belong to?

| Label | Phase |
|-------|-------|
| `phase: 0 bootstrap` | Go module, CI, Dockerfile, tooling |
| `phase: 1 document` | Skills Document engine |
| `phase: 2 gateway` | MCP Gateway + Redis transport |
| `phase: 3 identity` | Ed25519 identity + trust graph |
| `phase: 4 negotiation` | Negotiation protocol |
| `phase: 5 p2p` | Gossip P2P transport |
| `phase: 6 evolution` | Agent genomes + evolution |

**Component** — which package is primarily affected?

`component: document`, `component: protocol`, `component: transport`, `component: identity`, `component: negotiation`, `component: gateway`, `component: cli`, `component: ci`

**Status** — lifecycle state (set by the workflow, not you):

| Label | Set when |
|-------|---------|
| `status: in-progress` | `/ship` starts working on the issue |
| `status: blocked` | Cannot proceed — add a comment explaining why |
| `status: needs-review` | PR open, waiting for `/fix-review` |
| `status: stale` | No activity for 30+ days |

### Issue Templates

Three templates are available when opening an issue via the GitHub UI:

- **Task** — for feature/chore/research work (has type, priority, phase, component dropdowns)
- **Bug** — for broken behaviour (has reproduce steps, expected/actual, Go version)
- **Research/Spike** — for investigations (has time-box field, expected outcome)

Blank issues are disabled — always use a template.

### Issue Hygiene

- One concern per issue. If you find yourself writing "and also…" in the requirements, it's two issues.
- Acceptance criteria must be testable. "Code should be clean" is not a criterion. "go vet ./... passes" is.
- Close issues via the PR body (`Closes #N`), not manually.
- If an issue is blocked, add a comment explaining the blocker and add `status: blocked`. Don't leave it silently open.

---

## Milestones

Milestones map directly to the implementation phases:

| Milestone | Due | Gate criteria |
|-----------|-----|---------------|
| M1: First Document | Apr 6 2026 | Parse, validate, create all 11 message types. 100+ tests green. |
| M2: First Task | Apr 27 2026 | Claude Code submits task via MCP, receives result from another agent. |
| M3: Trusted Agents | May 18 2026 | All messages signed. Trust scores affect routing. |
| M4: First Negotiation | Jun 8 2026 | Two agents negotiate and reach agreement autonomously. |
| M5: No Redis | Jul 20 2026 | Agents communicate P2P without central infrastructure. |
| M6: First Spawn | Sep 14 2026 | New agent spawned from genome, passes sandbox, promoted to candidate. |

Issues are assigned to the milestone of the phase they belong to. If an issue's work is prerequisite to reaching a milestone's gate criteria, it belongs to that milestone.

---

## Working with Skills

Skills are invoked with the `Skill` tool, not by reading skill files. The tool loads the current version of the skill into context.

**Available skills:**

| Skill | Use when |
|-------|---------|
| `superpowers:brainstorming` | Before implementing anything non-trivial — mandatory design phase |
| `superpowers:writing-plans` | After brainstorming — decompose into bite-sized TDD tasks |
| `superpowers:test-driven-development` | Any time you're writing Go code (already in code-generator) |
| `superpowers:systematic-debugging` | A bug has resisted 2+ fix attempts |
| `superpowers:subagent-driven-development` | Implementing a multi-task plan in parallel |
| `mcp-server-dev:build-mcp-server` | Phase 2 — building the MCP Gateway |
| `aga2aga-protocol` | Working with Skills Documents, message types, lifecycle |
| `go-tdd` | Go-specific TDD conventions (adapts superpowers:test-driven-development) |
| `go-code-reviewer` | Reviewing Go code with aga2aga protocol compliance |

**The 1% rule:** If there is even a 1% chance a skill is relevant, invoke it. Do not rationalize away the check. The cost of loading a skill is low; the cost of working without it is high.

**Writing a new skill** (when you need project-specific guidance):

Follow the frontmatter template in CLAUDE.md. Description format: `[Brief capability]. Use when [trigger conditions].` Never put process steps in the description.

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

You can run all gates manually at any time:

```bash
go test ./... && go vet ./... && golangci-lint run
```

---

## DO_NOT_TOUCH Patterns

Patterns that must never be modified without explicit justification. Every review agent checks these before making changes.

| Pattern | File | Why |
|---------|------|-----|
| Lifecycle transition table | `pkg/document/lifecycle.go` | Spec §16 — changing breaks wire compatibility with live agents |
| Immutable genome fields list | `pkg/document/types_genome.go` | Spec §5.4 — these fields must survive agent mutation |
| `ValidTransition()` signature | `pkg/document/lifecycle.go` | Consumed by validator and CLI; signature change breaks callers |
| JSON Schema `$defs` names | `pkg/document/schema.yaml` | Used for `oneOf` dispatch; renaming silently breaks validation |
| `ProtocolVersion = "v1"` | `pkg/protocol/types.go` | Wire protocol constant — bump requires explicit migration plan |

**Adding a new DO_NOT_TOUCH pattern:**

When a pattern becomes load-bearing and must not drift, add it to:
1. `docs/WORKFLOW.md` DO_NOT_TOUCH table
2. A comment in the source file: `// DO_NOT_TOUCH: <reason> — see docs/WORKFLOW.md`

---

## Day-to-Day Patterns

### Starting a new session

The session-start hook injects context automatically. You'll see the current phase, open in-progress issues, and key constraints at the top of the conversation.

### Starting a new feature

```
/backlog I want to add X so that Y
```

Then review the issue that gets created. If the requirements look right, `/ship` it.

### Implementing something from the backlog

```
/ship         # auto-select
/ship 12      # specific issue
```

Leave it running. Check back when it's done or if it surfaces a question.

### Handling a tech-lead rejection

The tech-lead rejection message will say something like:

> REJECT: `pkg/document/parser.go` importing `internal/gateway` violates Clean Architecture.
> Corrected approach: move the shared type to `pkg/protocol/types.go` so both packages can import it without a cycle.

Read it, adjust your description, re-run `/backlog` with the revised scope.

### Handling a failed test during /ship

If `/ship` stops because tests are failing after 3 fix attempts, it surfaces the problem:

> 3 fix attempts failed. Probable architectural issue: the `As[T]` generic function cannot be used with the current YAML inline marshaling strategy. Consider switching to a two-pass parse or a separate typed parser per message type.

This is a design decision, not a bug. Read the diagnosis, decide the approach, then continue:

```
/ship 12      # re-run after you've addressed the diagnosis
```

### Reviewing a PR manually

```
/fix-review 14
```

### Making a change directly (without /ship)

1. Create a branch: `git checkout -b feat/12-my-change`
2. Write your code (follow TDD)
3. Create a PR: `gh pr create`
4. Run the review: `/fix-review`

### Debugging something

```
Skill superpowers:systematic-debugging
```

The skill walks through root cause investigation before proposing any fixes. Do not skip it — three failed fix attempts without a root cause is a bad outcome.

---

## Troubleshooting

### `/fix-review` says "provider: ask" on first run

The config hasn't been set. The skill will prompt you once:

```
Which provider?
  1. openrouter (requires OPENROUTER_API_KEY in .env.local)
  2. ollama (requires local models pulled)
```

Your choice is saved to `.claude/skills/fix-review/config.yaml` and used for all future runs.

### OpenRouter returns 401

Your API key is missing or wrong. Check `.env.local`:

```bash
cat .env.local | grep OPENROUTER
```

If missing, add `OPENROUTER_API_KEY=sk-or-...` and re-run.

### A round keeps triggering loop detection

Loop detection fires when ≥ 80% of the same file:line pairs appear in two consecutive rounds. This usually means:

- The external model is flagging something that *looks* like a bug but is a DO_NOT_TOUCH pattern
- A previous round's fix introduced a regression that the next round also flags

The Arbiter will DISMISS false positives. If it's a real issue, the Arbiter will CONFIRM and fix it with a different approach.

### go test fails after /fix-review commits

The Arbiter should have reverted any fix that broke tests. If it didn't, the revert check failed. Run:

```bash
go test ./...
```

Find the failing test. If the failure is in a file modified by a review round commit, use `git log --oneline` to find the offending commit and revert it:

```bash
git revert <commit-hash>
```

Then re-run `/fix-review` so the Arbiter can handle the issue differently.

### /ship picks the wrong issue

Check the labels on your issues. `/ship` picks based on phase labels matching the current milestone and priority ordering. If the wrong issue was picked, add `status: blocked` to it and re-run.

### tech-lead keeps rejecting the same design

The tech-lead rejection message includes a specific corrected approach. If the same issue comes up repeatedly, the design likely has a structural problem. Consider running `/review-arch` with the full package in scope to get a broader architectural review before proceeding.

### Skill not found

```
Skill superpowers:brainstorming   # fails with "skill not found"
```

The plugin is not installed. Run:

```
/plugin install superpowers@claude-plugins-official
```

---

## Reference

- Implementation plan: `/home/val/.claude/plans/noble-napping-church.md`
- Design docs (19 files): `context/preparation/`
- Formal protocol spec: `context/preparation/17-single_formal_spec_document.md`
- JSON Schema: `context/preparation/18-YAML_schema_companion.md`
- Reference implementations: `/home/val/wrk/github repos/0sel/`
- ClubTasker fix-review reference: `/home/val/wrk/oblabz/projects/ClubTasker/daily-clean-spark/.claude/skills/fix-review/`
