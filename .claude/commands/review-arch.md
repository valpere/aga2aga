# /review-arch

Invoke the tech-lead agent for standalone architecture review — without starting implementation.

**Usage:** `/review-arch [file-path | pr-number | description]`

---

## When to use

- Before starting a large feature to validate the approach
- When uncertain whether a package boundary decision is correct
- After a tech-lead REJECT in `/backlog`, to iterate on the design before re-submitting
- When refactoring package structure

## Steps

### 1. Resolve the target

**If a PR number is given:**
```bash
gh pr view {number} --repo valpere/aga2aga --json title,body,files
```

**If a file path is given:** Read the file(s).

**If a plain description is given:** Use it directly as the design input.

**If nothing is given:** Read recently modified files (`git diff --name-only HEAD~3`) to infer the current design.

### 2. Invoke tech-lead

Pass the resolved design context to the `tech-lead` agent:
- The description of what is being built
- Affected files and packages
- Any new interfaces, structs, or dependencies proposed

### 3. Apply or report outcome

**If APPROVE:** Report the approval with advisory notes. No action needed — implementation can proceed.

**If REJECT:** Present the rejection clearly:
- Specific violations (with file:line if known)
- Required corrections
- Recommended redesign

Suggested next step: revise the design and re-run `/review-arch`, or re-run `/backlog` with the corrected framing.

---

## Rules

- Never starts implementation — this command is read-only
- tech-lead runs on Opus; expect ~15s for complex designs
- Skip for `type: chore` and `type: docs` changes (no architecture to review)
