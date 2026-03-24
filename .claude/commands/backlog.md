# /backlog

Formalize a rough idea or request into a GitHub issue ready for implementation.

**Usage:** `/backlog [description of feature/bug/request]`

If no description is given, ask the user to describe what they want to add or fix.

## Steps

1. **Draft the issue** — invoke `pm-issue-writer` agent with the request as context:
   - Classify the request (Feature/Bug/Chore/Research/Docs)
   - Discover affected packages (read-only)
   - Verify aga2aga architectural constraints
   - Produce RFC 2119-compliant issue draft with acceptance criteria, labels, and milestone
   - **Output:** Draft text only — the agent does NOT create GitHub issues directly

2. **Architecture review** — skip for `type: chore` and `type: docs`; invoke `tech-lead` for all other types:
   - Input: issue draft text + affected file list
   - Output: APPROVE (with notes) or REJECT (with required changes)
   - If REJECT: return to step 1 with tech-lead feedback

3. **Create the issue** — the parent session (you, with Bash access) runs `gh issue create` using the pm-issue-writer draft:
   ```bash
   gh issue create \
     --repo valpere/aga2aga \
     --title "[Phase X.Y] <title>" \
     --milestone "<M1-M6>" \
     --label "<type>,<priority>,<phase>,<component>" \
     --body "<issue body>"
   ```

4. **Confirm** — print the created issue URL.

## Notes

- Do not create the GitHub issue until tech-lead approves (or type is chore/docs)
- If the request spans multiple phases or packages, split into separate issues
- DO_NOT_TOUCH patterns must be flagged in Implementation Notes if the issue touches them
