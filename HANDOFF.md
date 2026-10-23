# Maintainer Handoff

## Authoritative checkout

This Engram fork is based on upstream v3.3.0.
The installed executable and plugin caches are deployment artifacts, not source.
Do not recreate the removed `engram-src` duplicate worktree.

## Local behavior to preserve

- Minimal-memory mode skips automatic post-save candidate scans, bounds compact
  search results, and leaves implicit prompt capture off.
- Compact memory policy stays aligned across supported hosts. Codex prompt
  capture is opt-in; home-directory Claude sessions use shared ownership.
- `/dashboard`, browser CORS, `/projects/stats`, and expanded `/stats` remain.
- `ENGRAM_AGENT_CLI=pi` supports semantic conflict scans.
- `mem_update(project=...)` is **not supported**: its implementation was reverted
  in `c6ee4c2`. Do not recommend the old reassignment patch for this v3 checkout.

## Automatic author provenance

Observation writes accept runtime metadata, then inherit the registered session's
`author`, then consult Paseo runtime state and `ENGRAM_AUTHOR`. Unknown models
remain unknown; do not invent a precise model or backfill historical rows.

Claude/Codex Go hooks replace model-authored provenance on save, summary, passive
capture, and session registration. They resolve trusted hook metadata, bounded
transcript tails, and host configuration. Paseo's automatically generated launch
labels carry `ENGRAM_AUTHOR_SOURCE=paseo` so hooks can refresh them from live
agent state after a model change; explicitly configured overrides remain fixed.

Pi and OMP read provider/model from live execution context. The Pi adapter
propagates provenance through registration, normal and passive saves, summaries,
and compaction archives. `plugin/omp` contains the separate MCP + input-rewriting
extension integration; do not load the Pi adapter into OMP.

`POST /sessions` accepts optional `author`; nonempty registration refreshes it
without clearing it on omission or changing ownership/terminal-state rules.
`POST /sessions/{id}/summary` saves an observation without ending its session.
See `docs/author-provenance.md` for the supported contract.

## Verification and deployment

Source publication does not activate the installed runtime. See
[rollout status](docs/PLAN-auto-author-and-upstream-sync.md) for pending steps
and verification limits.

```powershell
go test ./cmd/engram ./internal/mcp ./internal/store ./internal/server ./internal/provenance
node --test plugin/pi/test/author-provenance.test.mjs plugin/omp/engram.test.mjs
```

On Windows, select Git Bash rather than the WSL launcher for shell-based tests.
After installing a binary, restart `engram serve` as well as MCP clients; replacing
an executable does not update running processes. Preserve live stores with a
SQLite online backup before migrations. Never commit binaries, databases,
credentials, or agent caches.

The upstream v3.5.2 rebase remains optional and has not been performed. Keep it
separate from provenance changes: shared compact-search projection, host binding,
HOME capture guards, and minimal-memory policy require conflict review.
