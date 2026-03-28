# Security Policy

## Reporting a Vulnerability

Please do **not** report security vulnerabilities via public GitHub issues.

Report vulnerabilities using [GitHub Security Advisories](https://github.com/valpere/aga2aga/security/advisories/new), or by emailing the maintainer directly (see the commit history for contact details).

Include: a description of the issue, the affected component, reproduction steps if applicable, and your assessment of the impact.

Expected response time: within 7 days for an acknowledgement; within 30 days for a fix or decision on whether the report is accepted.

## Supported Versions

The project is in pre-release (Phase 1). Only the `main` branch is supported.

| Version | Supported |
|---------|-----------|
| `main` (Phase 1) | Yes |
| Any tagged release | None yet |

## Trust Boundaries and Architecture

### Phase 1 (current)

There is no cryptographic authentication in Phase 1. All `Envelope.From` values are self-reported strings. No agent has a verified identity. The MCP Gateway (`cmd/gateway`) is a Phase 2 deliverable and is currently a stub.

The only software boundary in Phase 1 is the operator running `cmd/enveloper` locally against files they control.

### Phase 3 (planned)

`pkg/identity` will add Ed25519 keypairs, document signing, and signature verification. Once deployed, `Envelope.From` can be verified against `Envelope.Signature` + `Envelope.SigningKeyID`. Until then, every code path must treat `From` as unverified.

## Security Invariants

These invariants are enforced by the current code and must be maintained in all future changes.

### Envelope.From — unverified

`Envelope.From` is a self-reported wire string. It is present on the struct but carries no proof of identity.

**Rule:** Authorization decisions MUST NOT be based on `From` alone until Phase 3 Ed25519 verification is in place.

### Document.Extra — attacker-controlled

`Document.Extra` captures all YAML fields not defined in `Envelope`. Its keys and values are unvalidated and must be treated as attacker-controlled input on any network-facing path.

**Rule:** `Extra` MUST NOT be used directly for auth, signing, lifecycle state transitions, or routing decisions. Always use `As[T](doc)` to obtain a typed struct.

### As[T] envelope-key stripping (injection defense)

`As[T](doc)` strips all 13 envelope YAML keys from `Extra` before the YAML round-trip to type `T`. This prevents an attacker from crafting a document where `Extra` contains a key like `from` or `type` that would shadow the real envelope value inside the typed struct.

**Rule:** Never bypass `As[T]` to access `Extra` directly when building typed structs. (CWE-20)

### Wire-reported status fields (CWE-20)

The following fields are self-reported strings on the wire and must not be used as authoritative state without a state-store lookup:

| Field | Type | Status |
|-------|------|--------|
| `Promotion.FromStatus` | required on wire | self-reported |
| `Promotion.ToStatus` | required on wire | self-reported |
| `Rollback.FromStatus` | required on wire | self-reported |
| `Rollback.ToStatus` | required on wire | self-reported |
| `Quarantine.FromStatus` | optional on wire | self-reported |
| `Retirement.FromStatus` | optional on wire | self-reported |
| `Promotion.Reason` | wire string | opaque label only |
| `Quarantine.Reason` | wire string | opaque label only |
| `Retirement.Reason` | wire string | opaque label only |

**Rule:** Executors MUST derive authoritative state from the state-store and call `document.ValidTransition(from, to)` — never trust wire values. When `from_status` is absent (quarantine/retirement), the orchestrator MUST perform a state-store lookup before calling `ValidTransition`.

### Self-action governance

Agents must not be able to promote, rollback, quarantine, or retire themselves.

**Rule:** The semantic validator (`ValidateSemantic`) denies `from == target_agent` for all four governance types — promotion, rollback, quarantine, and retirement. This check runs independently of whether `from_status` is present on the wire.

### GenomePatch — DO_NOT_TOUCH fields structurally absent

`SpawnProposal.GenomePatch` is typed as `*GenomePatch`, which contains only the mutable subset of genome fields. Immutable fields (`id`, `lineage`, `genome_version`, `created_at`, `kind`) are not present in `GenomePatch` at all.

**Rule:** Patch-apply logic MUST only append to `SoftConstraints`, never replace them. DO_NOT_TOUCH fields are not available via the typed struct.

### PromptPolicy.Style — open vocabulary

`AgentGenome.PromptPolicy.Style` is `map[string]any` — an open vocabulary per spec §4.3. Its keys and values arrive from the wire.

**Rule:** Callers MUST sanitize `PromptPolicy.Style` before using any value for auth, signing, or lifecycle decisions. (Annotated in `types_genome.go`.)

### EscalationRule — code execution and routing (CWE-20, CWE-601)

`EscalationRule.Condition` is an opaque label string describing when escalation applies.

**Rule:** `Condition` MUST NOT be executed or interpreted as code, a query language, or a template. It is a human-readable label only. (CWE-20)

`EscalationRule.Target` is a self-reported agent identifier.

**Rule:** `Target` MUST be validated against the agent registry before any dispatch or delegation. Do not dispatch to an unregistered target. (CWE-601)

### RoutingPolicy — attacker-controlled wire strings

`AgentGenome.RoutingPolicy.Accepts`, `DelegatesTo`, and `EscalationRules` all arrive from the wire.

**Rule:** Dispatchers MUST sanitize `Accepts` against the protocol registry before routing. `DelegatesTo` IDs MUST be validated in the state-store before any delegation decision. (Annotated in `types_genome.go`.)

### RecombineProposal — unverified identifiers

`RecombineProposal.CandidateID` and `ParentIDs` are self-reported.

**Rule:** Executors MUST verify both fields against the state-store before creating a new genome.

## Size Limits

`document.MaxDocumentBytes = 65536` (64 KiB) is a hard limit enforced in `Parse` and in `readAndParseFile`. This prevents YAML billion-laughs attacks and limits the blast radius of a malformed document reaching the gateway.

The internal intermediate representation is also capped at `4 * MaxDocumentBytes` (256 KiB) to bound memory during YAML-to-JSON conversion.

**Reference:** CWE-400 (Uncontrolled Resource Consumption)

## Known Limitations (Phase 1)

The following are known gaps that will be addressed in later phases:

| Limitation | Resolution phase |
|------------|-----------------|
| No cryptographic identity for `Envelope.From` | Phase 2.5 (agent keys — lightweight) / Phase 3 (Ed25519 — full) |
| No transport-layer encryption | Phase 2+ (TLS on Redis; Phase 5: P2P) |
| No rate limiting on document ingestion | Phase 2 (gateway) |
| `NegotiationTransition` is a stub (always returns false) | Phase 4 |

### Phase 2.5 — Agent API Key Authorization (current)

`cmd/gateway` now supports `--require-agent-key`. When enabled, every MCP tool call must carry an `api_key` parameter whose SHA-256 hash matches an active `role=agent` API key in the admin store, and whose bound agent ID matches the claimed `agent` parameter.

This provides lightweight identity binding without cryptography. It is a bridge until Phase 3 Ed25519 is in place.

**Trust model:** The key binding is as strong as the admin store's access controls. If the admin database is compromised, attacker-issued keys can impersonate any agent. Keep the admin DB access-controlled and the gateway's `--admin-db` path non-symlinked (the gateway applies `filepath.EvalSymlinks` guard — CWE-22/61).

**Revocation:** Keys can be revoked in the Admin UI at any time. The next call with a revoked key returns an authentication error immediately.

## CWE Reference

| CWE | Description | Where applied |
|-----|-------------|---------------|
| CWE-20 | Improper Input Validation | Extra fields, wire-reported status, EscalationRule.Condition |
| CWE-22 | Path Traversal | `readAndParseFile` uses `filepath.EvalSymlinks` before `os.Open` |
| CWE-61 | UNIX Symbolic Link Following | Same: `filepath.EvalSymlinks` guard |
| CWE-200 | Information Exposure | `filepath.Base(path)` in all error and success messages |
| CWE-252 | Unchecked Return Value | `Signer.Verify` returns `(bool, error)` — non-nil error is indeterminate |
| CWE-400 | Uncontrolled Resource Consumption | `MaxDocumentBytes` limit in `Parse` and `readAndParseFile` |
| CWE-601 | URL Redirection / Open Redirect | `EscalationRule.Target` must be validated in agent registry |
