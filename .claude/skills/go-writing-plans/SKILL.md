---
name: go-writing-plans
description: "Go implementation plan writer for aga2aga. Use when you have a GitHub issue or spec for a multi-step Go task, before touching code."
metadata:
  version: "1.0.0"
  domain: protocol
  triggers: plan, implementation plan, writing plan, design, before coding, spec
  scope: design, implementation
---

# Go Writing Plans — aga2aga

**Announce at start:** "I'm using the go-writing-plans skill to create the implementation plan."

This skill adapts `superpowers:writing-plans` for Go and the aga2aga codebase.
The plan structure, scope check, and review loop are unchanged — only tooling and conventions are Go-specific.

---

## Before Starting

1. Read the GitHub issue fully
2. Read any referenced design docs in `../context/preparation/`
3. Check the approved implementation plan (`~/.claude/plans/`) for build-order dependencies
4. Identify which packages are affected — stay within Clean Architecture boundaries:
   - `pkg/` → public library, no `internal/` or `cmd/` imports
   - `internal/` → private, no `cmd/` imports
   - `cmd/` → thin entry points only

---

## Scope Check

If the issue covers multiple independent subsystems, flag it before planning.
Each plan must produce independently compilable, `go test ./...`-passing software.

---

## File Structure

Map affected files before writing tasks:

```
pkg/document/
  parser.go          # create — SplitFrontMatter, Parse, ParseAs, Serialize
  parser_test.go     # create — table-driven tests against testdata/
tests/testdata/
  valid_genome.md    # create — fixture
  invalid_no_frontmatter.md  # create — fixture
```

Rules:
- One clear responsibility per file
- Test file lives alongside production file (`foo.go` → `foo_test.go`)
- Fixtures go in `tests/testdata/` — name describes the case
- Follow existing naming in the package (check before creating)

---

## Plan Document Header

**Every plan MUST start with:**

```markdown
# [Feature Name] Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: go-tdd for each implementation step.

**Goal:** [One sentence]

**Architecture:** [2–3 sentences on approach and Clean Architecture layer]

**Tech Stack:** Go 1.24, gopkg.in/yaml.v3, [other relevant deps]

**Build dependencies:** [list issues this depends on, e.g. "requires #15"]

---
```

---

## Task Structure (Go edition)

````markdown
### Task N: [Component Name]

**Files:**
- Create: `pkg/document/parser.go`
- Create: `pkg/document/parser_test.go`
- Modify: `tests/testdata/` (add fixtures)

- [ ] **Step 1: Write the failing test**

```go
func TestParse_ValidGenome(t *testing.T) {
    raw, err := os.ReadFile("../../tests/testdata/valid_genome.md")
    require.NoError(t, err)

    doc, err := Parse(raw)

    require.NoError(t, err)
    assert.Equal(t, protocol.MessageTypeAgentGenome, doc.Type)
}
```

- [ ] **Step 2: Run test — verify RED**

```bash
go test -count=1 -run TestParse_ValidGenome ./pkg/document/...
```

Expected: `FAIL` — `Parse` undefined (or similar compile/runtime failure)

- [ ] **Step 3: Write minimal implementation**

```go
func Parse(raw []byte) (*Document, error) {
    // ... minimal code to pass
}
```

- [ ] **Step 4: Run test — verify GREEN**

```bash
go test -count=1 -run TestParse_ValidGenome ./pkg/document/...
```

Expected: `PASS`

- [ ] **Step 5: Run full suite**

```bash
go test ./... && go vet ./...
```

Expected: all pass, no vet warnings

- [ ] **Step 6: Commit**

```bash
git add pkg/document/parser.go pkg/document/parser_test.go tests/testdata/valid_genome.md
git commit -m "feat(document): add Parse() with valid_genome fixture"
```
````

---

## Commit Conventions

```
<type>(<scope>): <description>

Types: feat, fix, test, refactor, chore, docs
Scope: document, protocol, lifecycle, validator, builder, cli, transport
```

Examples:
- `feat(document): add SplitFrontMatter and Parse`
- `test(lifecycle): add table-driven ValidTransition tests`
- `chore: go mod tidy after adding testify`

Commit after each GREEN + passing suite — not at the end of the whole task.

---

## Verify Step (after each task)

```bash
go build ./... && go vet ./... && go test ./...
```

All three must pass before moving to the next task.

---

## Remember

- Exact file paths always (package-relative: `pkg/document/parser.go`)
- Complete Go code in the plan — not "add validation here"
- Exact commands with expected output
- `go test -count=1 -run TestX ./pkg/...` to disable cache and target
- Reference DO_NOT_TOUCH patterns from CLAUDE.md before touching lifecycle/schema/protocol code
- DRY, YAGNI, TDD, frequent commits

---

## Plan Review Loop

After writing the complete plan:

1. Dispatch a plan-reviewer subagent with: path to plan, path to issue, path to relevant design doc
2. If issues found: fix and re-dispatch
3. If approved: offer execution choice

Max 3 review iterations — surface to user if stuck.

---

## Execution Handoff

After saving:

**"Plan complete. Two execution options:**

**1. Subagent-Driven (recommended)** — fresh subagent per task, review between tasks
Use: `superpowers:subagent-driven-development`

**2. Inline** — execute in this session with checkpoints
Use: `superpowers:executing-plans`

**Which approach?"**
