---
name: go-tdd
description: "Go-specific TDD workflow for aga2aga. Use when implementing any feature, bug fix, or behaviour change in Go code."
metadata:
  version: "1.0.0"
  domain: protocol
  triggers: tdd, test, failing test, red-green, implement, feature, bugfix
  scope: implementation, testing
---

# Go TDD — aga2aga

**Announce at start:** "I'm using the go-tdd skill."

This skill adapts `superpowers:test-driven-development` for Go and the aga2aga codebase.
The Iron Law and Red-Green-Refactor cycle are unchanged — only the tooling is Go-specific.

---

## The Iron Law

```
NO PRODUCTION CODE WITHOUT A FAILING TEST FIRST
```

Write code before the test? Delete it. Start over. No exceptions.

---

## Red-Green-Refactor (Go edition)

### RED — Write the Failing Test

Default pattern: **table-driven tests**.

```go
func TestFoo(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    string
        wantErr bool
    }{
        {name: "valid input", input: "ok", want: "ok", wantErr: false},
        {name: "empty input", input: "", want: "", wantErr: true},
    }
    for _, tc := range tests {
        t.Run(tc.name, func(t *testing.T) {
            got, err := Foo(tc.input)
            if (err != nil) != tc.wantErr {
                t.Errorf("Foo() error = %v, wantErr %v", err, tc.wantErr)
            }
            if got != tc.want {
                t.Errorf("Foo() = %q, want %q", got, tc.want)
            }
        })
    }
}
```

**Requirements:**
- One behaviour per subtest
- Names describe behaviour, not implementation (`"rejects empty id"` not `"test1"`)
- Use real code — mocks only when unavoidable (e.g., `transport.Transport` interface)
- Assertions: `t.Errorf` / `testify/assert` — NO jest matchers

### Verify RED — Watch It Fail

**MANDATORY. Never skip.**

```bash
# Run single test (disables cache so you always see real output)
go test -count=1 -run TestFoo ./pkg/document/...

# Run specific subtest
go test -count=1 -run "TestFoo/rejects_empty_id" ./pkg/document/...
```

Confirm:
- Output contains `FAIL` + a meaningful message (not a compile error)
- Fails because the feature is missing, not because of a typo or missing import
- Failure message is the one you expect

**Test passes?** You are testing existing behaviour. Fix the test.
**Compile error?** Fix it — a compile error is not RED, it is broken. Fix until it compiles and fails.

### GREEN — Minimal Code

Write the simplest Go that makes the test pass. No YAGNI violations.

```go
// Good — just enough
func Foo(input string) (string, error) {
    if input == "" {
        return "", errors.New("input is empty")
    }
    return input, nil
}

// Bad — over-engineered
func Foo(input string, opts ...FooOption) (string, error) { ... }
```

### Verify GREEN

```bash
go test -count=1 -run TestFoo ./pkg/document/...
```

Confirm:
- Named test passes
- **Full suite still green:** `go test ./...`
- No new `go vet` warnings: `go vet ./...`

### REFACTOR

After green only. Keep tests green. Do not add behaviour.

- Remove duplication
- Improve names
- Extract helpers if used in >1 place

---

## aga2aga-Specific Rules

### Package conventions
- Tests live in `_test.go` files in the same package (white-box) or `package foo_test` (black-box)
- Test fixtures (`.md` documents) live in `tests/testdata/`
- Use `testify/assert` for readability on complex structs; plain `t.Errorf` for simple comparisons

### What to test
- Parser: round-trip invariant `Parse(Serialize(doc)) == doc`
- Lifecycle: every valid transition returns `true`; representative invalid ones return `false`
- Validator: valid docs → zero errors; invalid docs → specific `ValidationError` entries
- Builder: `Build()` with missing required fields returns error, not panic

### DO NOT mock
- `pkg/document` types — use real structs
- `pkg/protocol` registry — use the real registry

### Interfaces you MAY mock (only when testing callers)
- `transport.Transport` — acceptable to stub in gateway tests
- `identity.Signer` — acceptable to stub in signing tests

---

## Commands Reference

| Action | Command |
|--------|---------|
| Run all tests | `go test ./...` |
| Run package tests | `go test ./pkg/document/...` |
| Run single test (no cache) | `go test -count=1 -run TestFoo ./pkg/document/...` |
| Run subtest | `go test -count=1 -run "TestParser/valid_genome" ./pkg/document/...` |
| Verbose output | `go test -v -count=1 -run TestFoo ./pkg/document/...` |
| Race detector | `go test -race ./...` |
| Vet | `go vet ./...` |

---

## Verification Checklist

Before marking work complete:

- [ ] Every new function has at least one table-driven test
- [ ] Watched each test fail with `FAIL` before implementing
- [ ] Each test failed for the right reason (feature missing, not typo)
- [ ] Wrote minimal Go to pass — no YAGNI
- [ ] `go test ./...` passes
- [ ] `go vet ./...` clean
- [ ] No mocks where real types work

---

## Red Flags — Stop and Start Over

- Code written before test
- Test added after implementation
- Test passes immediately (never saw RED)
- Compile error accepted as "RED"
- "I'll write tests after"
- "Too simple to test"
- Any rationalization

All of these mean: delete production code, start over with the failing test.
