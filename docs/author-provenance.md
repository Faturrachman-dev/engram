# Automatic author provenance

Agents should omit `author` in memory arguments. Supported hooks/adapters inject
`harness/provider/model` (or `harness/model` when no provider is available).
They replace untrusted model-supplied labels and refresh from live host metadata.
Missing exact model evidence stays unknown, rather than claiming 100% model
identification from a config alias or inventing a fallback model.

## Write precedence

For `mem_save`, `mem_session_summary`, `mem_capture_passive`, and the equivalent
HTTP observation endpoints:

1. Nonempty runtime-injected `author`.
2. Parent session's persisted author.
3. Live Paseo agent state when `PASEO_AGENT_ID` is present.
4. `ENGRAM_AUTHOR` in the serving process environment.

Session registrations accept optional `author`, update it when nonempty, and do
not erase it on omission. Ownership validation and terminal-session rules still
apply atomically. Existing observations are not retroactively relabeled. A topic
revision adopts its current author's label; duplicate acknowledgements and
idempotent replays retain the original committed provenance.

HTTP supports `POST /sessions`, `POST /observations`,
`POST /observations/passive`, and `POST /sessions/{id}/summary`.
The last route requires `content`, accepts `project` and `author`, defaults the
project to the existing session owner, and does not end the session.

## Host evidence

- **Claude Code:** explicit `ENGRAM_AUTHOR`, Paseo state, host `model`, bounded
  transcript tail's last assistant model, `ANTHROPIC_MODEL`, then settings model.
- **Codex:** explicit `ENGRAM_AUTHOR`, Paseo state, host `model`, supplied
  transcript or a matching rollout's last model, then top-level config model and
  provider. `CODEX_HOME` and `CLAUDE_CONFIG_DIR` are honored.
- **Pi:** live context provider and model, with `ENGRAM_AUTHOR` only when no model
  is exposed. Provider prefixes are not duplicated when embedded in the model ID.
- **OMP:** the separate `plugin/omp` adapter reads live execution context and
  rewrites Engram MCP input. It does not guess from unrelated/latest sessions.

Paseo state supports both flat and cwd-grouped agent records under
`$PASEO_HOME/agents` (default `~/.paseo/agents`). Automatic Paseo launch env uses
`ENGRAM_AUTHOR_SOURCE=paseo`; hooks then prefer live state over that static
launch label after model switches. Do not set this marker for custom overrides.

## Deployment and limits

Rebuild the Go binary, restart the HTTP server, and start fresh MCP processes.
Update/load the source Pi adapter separately from the binary. OMP setup is in
`plugin/omp/README.md`; its MCP and extension APIs differ from Pi's.

Direct HTTP/MCP callers without a trusted adapter can supply arbitrary explicit
labels: author metadata is not an authentication or authorization boundary.
Shared server environment cannot identify a missing caller model. No transcript,
agent identifier, credentials, or raw runtime state is persisted as provenance.
