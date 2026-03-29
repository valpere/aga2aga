# Skill: /live-test
# aga2aga — Interactive Admin UI Testing

---

## OVERVIEW

A conversational browser testing session against the aga2aga Admin UI.
Drive the browser, observe the live result, log issues, then generate a report.

```
/live-test               → open local admin UI, start interactive session
/live-test <url>         → open a specific URL instead
/live-test report        → generate session report right now
```

---

## STEP 0: Resolve Target URL

Parse the argument:
- No argument → `URL=http://localhost:8087`
- `report` → skip to STEP 4 immediately
- Any other value → treat as the target URL

---

## STEP 1: Open the Browser

Navigate to the URL:
```
browser_navigate(url)
```

Take a screenshot to confirm the page loaded.

Then summarise what's on screen and prompt for the first action:

> "Browser open at [URL]. What would you like to test?
> Examples: 'log in as admin', 'create an agent key', 'add a policy', 'check the audit log'.
> Say **'done'** or **'report'** when finished."

---

## STEP 2: Interactive Loop

Repeat until the user says "done", "finish", "report", or "stop".

### On each message:

**Narrate before acting:**
> "Clicking the 'Create Key' button."
> "Selecting role → agent and filling in the Agent ID field."

**Act — tool mapping:**

| Instruction | Tools |
|---|---|
| "go to / navigate to X" | `browser_navigate` |
| "click X" / "press X" | `browser_snapshot` → `browser_click` |
| "type X" / "fill in X" | `browser_snapshot` → `browser_type` |
| "scroll down / up" | `browser_scroll` |
| "select X" / "choose X" | `browser_snapshot` → `browser_select_option` |
| "press Enter / Tab / Escape" | `browser_press_key` |
| "go back" | `browser_go_back` |
| "what's on screen?" | `browser_snapshot` → describe |
| "screenshot" / "capture" | `browser_take_screenshot` |
| "check console / errors" | `browser_console_messages` |
| "log in as admin" | see credentials below |

**Describe what happened after acting:**
> "Policy saved — redirected to the policies list; new entry visible."
> "401 Unauthorised — the API key was rejected."

**Screenshot automatically after:**
- Login / logout
- Any page navigation
- Any form submission
- Any error or unexpected behaviour

**Log an issue immediately** when something looks wrong:

```
ISSUE #{n}: {title}
URL:      {current URL}
Steps:    {what was done}
Expected: {what should happen}
Actual:   {what happened}
Severity: {critical | major | minor | cosmetic}
Screenshot: taken ✓
```

**Ask when ambiguous** — never guess on a click that could change data.

---

## STEP 3: Credentials

| Role | Username | Password |
|---|---|---|
| Admin (default) | `admin` | `changeme` (or read `ADMIN_PASSWORD` from `.env.local`) |

If the password has been changed and `.env.local` does not contain `ADMIN_PASSWORD`, ask:
> "What's the current admin password?"

---

## STEP 4: Generate Report

When the user says "done", "report", "finish", or "stop":

### 4a. Generate Playwright test (if available)

```
browser_generate_playwright_test
```

If the tool is unavailable, skip this step and note it in the report.

Save the output to:
```
e2e/interactive/{YYYY-MM-DD-HHmm}.spec.ts
```

### 4b. Markdown report

```markdown
## Live Test Report — {date and time}

**Environment:** {URL}
**Flows tested:** {count}
**Issues found:** {count}

---

### Flows tested

| # | Flow | Result |
|---|------|--------|
| 1 | {description} | ✅ Pass / ⚠️ Issue / ❌ Fail |

---

### Issues found

{If none: "No issues found during this session."}

#### Issue 1: {title}
- **URL:** {URL at time of issue}
- **Steps to reproduce:**
  1. {step}
- **Expected:** {what should happen}
- **Actual:** {what happened}
- **Severity:** {critical | major | minor | cosmetic}
- **Screenshot:** {filename or "taken ✓"}

---

### Generated test file

{If generated: "Saved to e2e/interactive/{filename}.spec.ts"}
{If skipped: "browser_generate_playwright_test unavailable — no test file generated."}

---

### Suggested GitHub issues

{For each issue with severity critical or major:}
- [{severity}] {title} — {one-sentence description of the fix}
```

### 4c. Offer to create GitHub issues

> "Would you like me to open GitHub issues for the critical/major findings?"

If yes, use `gh issue create` for each critical or major issue with label `bug`.

---

## RULES

1. **No destructive actions** — do not delete data or revoke keys unless explicitly asked and confirmed.
2. **Narrate before acting** — one sentence.
3. **Screenshot after every significant state change.**
4. **Ask when ambiguous** — never guess on a destructive click.
5. **Keep the issue log running** — add immediately when flagged.
6. **Report is mandatory** — always generate the Markdown report, even with zero issues.
7. **Offer GitHub issue creation** — always ask at the end.
