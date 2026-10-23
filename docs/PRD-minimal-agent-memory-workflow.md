# Minimal agent memory workflow

## Status and intent

The core workflow is implemented in this fork. Deployment must be checked in the
actual host; this document does not certify installed binaries or fresh-session
behavior. Work first; remember meaningful outcomes, not a chronological activity
log. Preserve identity, project routing, privacy, and explicit conflict review.

## Contract

- Reuse sufficient injected context. Retrieve only when relevant information is
  missing; prefer one targeted search and fetch full observations only as needed.
- Save durable decisions, preferences, verified fixes, and meaningful operational
  state. Group related outcomes under stable topics; do not save routine probes,
  formatting, test passes, or facts cheaply recoverable from maintained code.
- Summarize at genuine close/compaction, not after ordinary replies. Reuse the
  registered runtime session identity; never invent one or replace hook ownership.
- Keep one canonical compact policy across setup, MCP, skills, and host adapters.
  A memory-service failure must not prevent completion of the user's task.
- Minimal mode skips ordinary post-save candidate scans and returns
  `judgment_required=false`, `conflict_review="manual"`. Explicit conflict review
  remains available. Legacy mode retains automatic review behavior, not unsafe
  implicit prompt capture. Do not delete or silently judge existing relations.
- Search defaults to compact in minimal mode and legacy format in legacy mode.
  Compact results omit relation histories unless explicitly requested and avoid
  querying omitted histories. Explicit `include_relations` overrides that default.
- Compact MCP search uses an 8 KiB serialized-payload budget; explicit `max_bytes`
  accepts 1–64 KiB. Keep JSON valid, identity intact, and omissions/truncation
  visible. Legacy output is unbudgeted unless a budget is explicitly requested.
- Raw prompt capture remains deliberately opt-in, including `mem_save`. Saving a
  sanitized observation is not permission to retain the original prompt.
- Preserve bounded startup/compaction context, narrow write-tool hooks, terminal
  session rules, project ownership, and headless Windows execution. No broad
  per-read binding hooks, new capture daemons, or background semantic jobs.

## Compatibility and verification

Select MCP behavior with `--memory-mode=minimal|legacy` or `ENGRAM_MEMORY_MODE`;
explicit CLI selection wins. Invalid values fail rather than silently changing
behavior. Inspect active host config instead of assuming a cached plugin version.

Verify mode precedence, opt-in capture, scan bypass, explicit review, compact
byte/UTF-8 bounds, omitted relation queries, policy synchronization, and session
safety with synthetic data. Focused checks should include `internal/mcp`,
`internal/store`, `internal/setup`, `internal/memorypolicy`, and `cmd/engram`.
Report baseline/platform failures separately; do not weaken assertions to pass.

Fresh-session checks must show simple tasks need no memory calls, past decisions
use targeted retrieval, coherent fixes produce coherent memories without automatic
judgments, and actual compaction retains identity. Unit tests alone do not prove
these host behaviors or establish a percentage reduction in tokens/tool calls.

## Deployment and rollback

Back up active binaries and affected configuration outside generated caches.
Deploy maintained source, regenerate policy/setup, reload hosts, and review hook
trust where required. Back up SQLite consistently if a migration is needed.
Restore backed-up artifacts for rollout failures; legacy mode is a compatibility
option, not permission to re-enable raw capture or delete real work.

See [HANDOFF.md](../HANDOFF.md) for fork constraints and
[author-provenance.md](author-provenance.md) for automatic attribution.
