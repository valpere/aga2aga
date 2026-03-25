# Contributing

aga2aga is a solo-developer project, but contributions are welcome. This document describes how the project is developed and what is expected of contributors.

## Prerequisites

- **Go 1.24+** — `go version` must show `go1.24` or later
- **gh CLI** — used for issue management and PR creation
- **golangci-lint** — v2.11.4 is used in CI. Local machines typically have v1; do **not** use local golangci-lint to validate the `.golangci.yml` config (the v2 schema is incompatible with v1). Run `go vet ./...` locally instead.

## Getting Started

```bash
git clone https://github.com/valpere/aga2aga.git
cd aga2aga
go mod tidy
go build ./...
go test ./...
go vet ./...
```

All four commands must exit zero before you start writing code.

## Test-Driven Development

This project follows strict TDD. The rules are not negotiable:

1. **Write the failing test first.** Run it and confirm it outputs `FAIL` — not a compile error, not a panic.
2. **Write the minimum code to pass.** No extra abstractions, no YAGNI violations.
3. **Run the full suite.** `go test ./...` must be green before committing.

Default pattern is table-driven tests:

```go
func TestFoo(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    string
        wantErr bool
    }{
        {name: "valid", input: "ok", want: "ok"},
        {name: "empty rejects", input: "", wantErr: true},
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

Use `errors.Is` for sentinel error assertions. Never use `strings.Contains(err.Error(), ...)`.

To run a single test without the cache:

```bash
go test -count=1 -run TestFoo ./pkg/document/...
```

## Code Style

- **Format:** `gofmt -w .` — enforced by CI.
- **Vet:** `go vet ./...` — must be clean before any PR.
- **Copy-on-read:** Functions returning slices from shared state (registry, transition table) return copies. Callers must not be able to mutate shared data.
- **sync.Once singletons:** Process-wide singletons (e.g., `DefaultValidator()`) use `sync.Once` with eager initialization at `init()` time.
- **Error wrapping:** All errors use `%w` for `errors.Is` compatibility. No `fmt.Errorf` without `%w` when wrapping an existing error.
- **No emojis** in source files or documentation.

## Commit Conventions

This project uses [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<scope>): <description>
```

Types: `feat`, `fix`, `test`, `refactor`, `chore`, `docs`

Scope matches the affected package: `document`, `protocol`, `lifecycle`, `validator`, `builder`, `cli`, `transport`, `identity`, `negotiation`, `gateway`, `ci`

Examples:

```
feat(document): add Parse() with valid_genome fixture
fix(cli): use filepath.Base in validate success output
chore(ci): add coverage reporting and go mod tidy drift check
docs: add ARCHITECTURE.md and SECURITY.md
```

Commit after each RED→GREEN cycle, not at the end of a large feature.

## Pull Request Process

1. Open a GitHub issue first. No orphan branches.
2. Create a branch: `feat/{number}-{slug}`, `fix/{number}-{slug}`, or `chore/{number}-{slug}`.
3. Implement following TDD. All commits must leave `go test ./...` green.
4. `go vet ./...` must be clean.
5. Push and open a PR using `gh pr create`.
6. CI must pass (golangci-lint v2, tests, go mod tidy check). The repo does **not** enforce CI as a required merge gate — do not merge with failing CI.

The project uses a 4-round AI-assisted code review process (described in [docs/WORKFLOW.md](docs/WORKFLOW.md)). If you are not using the Claude Code tooling, a standard peer review is fine.

## DO_NOT_TOUCH Rules

Some parts of the codebase are marked `DO_NOT_TOUCH`. These markers protect spec §16 wire compatibility and are not style suggestions.

**What is protected and why:**

| Protected item | Reason |
|----------------|--------|
| `ValidTransition()` function signature in `lifecycle.go` | Callers across the codebase depend on this exact interface; changing it would require coordinated updates to all orchestrators |
| Lifecycle transition table (the `transitionTable` map) | Encodes spec §16 exactly; any change breaks compatibility with agents running different versions |
| Immutable genome fields list (`id`, `lineage`, `genome_version`, `created_at`, `kind`) | Spec §5.4 immutability contract; changing these fields invalidates signatures |
| JSON Schema `$defs` names in `schema.yaml` | `SchemaRef` values in the registry map to these names; renaming breaks schema lookup |
| `ProtocolVersion = "v1"` constant | Wire constant; changing it breaks all existing agents |
| `constraints.hard` and `identity` genome fields | Proposals that weaken hard constraints or alter identity must be rejected by the governance layer |

If you believe a DO_NOT_TOUCH item needs to change, open an issue and discuss the wire compatibility impact before touching the code.

## Project Structure

```
cmd/gateway/      MCP Gateway binary (Phase 2)
cmd/aga/          CLI tool (validate, create, inspect)
pkg/document/     Skills Document parser, validator, builder, lifecycle
pkg/protocol/     Message type constants and registry
pkg/transport/    Transport interface (Redis/Gossip implementations: future)
pkg/identity/     Ed25519 identity (Phase 3)
pkg/negotiation/  Negotiation state machine (Phase 4)
internal/gateway/ MCP Gateway internals (Phase 2)
tests/testdata/   Test fixture documents
schemas/          Standalone JSON Schema (reference copy)
docs/             Extended documentation
```

See [ARCHITECTURE.md](ARCHITECTURE.md) for the detailed architecture.

## License

MIT. By contributing you agree that your changes will be released under the same license.
