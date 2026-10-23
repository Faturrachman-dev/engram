# Automatic author provenance: rollout and upstream sync

## Status

Source implementation is complete on the v3.3.0 fork. See
[author-provenance.md](author-provenance.md) for the supported API, precedence,
and host evidence; [HANDOFF.md](../HANDOFF.md) covers fork invariants.

The last verified rollout staged a new binary but did not install it or restart
the live HTTP/MCP processes. Pi and OMP adapters were installed locally but
require host reloads. Paseo launch-metadata changes live in its separate checkout;
its dependency-based verification and deployment remain pending. Source publication
is not evidence of runtime activation. Rediscover actual versions before deploying.

## Activation checklist

1. Run the focused Go and Pi/OMP attribution suites listed in the handoff.
2. Identify the active executable, store, HTTP listener, and MCP clients. Take a
   consistent SQLite online backup before migration; do not copy a live DB alone.
3. Build/install from this checkout, then restart the intended HTTP/MCP processes.
   Reload Pi/OMP with their maintained adapters. Do not restart unrelated services.
4. In fresh host sessions, verify registration and saves without model-authored
   labels, model-switch refresh, summary/passive attribution, and ownership checks.
5. Verify Paseo separately before deploying it. Do not infer live-provider success
   from isolated metadata fixtures.

Earlier focused Go packages and isolated binary smoke reported PASS. Windows
aggregate runners required termination after package completion. Focused Pi/OMP
attribution tests passed; the broader Pi suite had baseline policy/timer failures.
No billable provider conversations were run. These are historical results, not
claims that the entire suite or deployment is green.

Publication checks (2026-10-23): focused attribution checks passed in all five
affected Go packages; both Pi/OMP adapter tests passed. The broader affected-package
run exceeded its 180-second limit, so full-suite success is not established.

## Optional upstream reconciliation

Keep the previously reviewed v3.5.2 sync separate from provenance. It has not been
performed; the old upstream review is not a claim about today's latest release.
Before rebasing, inspect current upstream history and reconcile:

- Compact-search projection with the fork's minimal-mode candidate-scan bypass.
- Shared memory policy without compulsory per-turn bookkeeping.
- Host session binding and HOME guards without losing shared HOME ownership.
- Windows process/SQLite fixes without weakening identity or privacy boundaries.

Do not backfill historical authors, purge memories, or revive the reverted
`mem_update(project=...)` implementation as part of either rollout.
