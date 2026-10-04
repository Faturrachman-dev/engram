---
name: engram-memory-protocol
description: >
  Persistent memory discipline for Engram contributors.
  Trigger: Decisions, bugfixes, discoveries, preferences, or session closure.
license: Apache-2.0
metadata:
  author: gentleman-programming
  version: "2.0"
---

## Engram memory — compact policy

Work first; remember meaningful outcomes. Do not search, save, or judge before
every action.

### Retrieve when it adds missing information

Use context already injected or in the conversation; do not refetch it
because a turn begins. Retrieve when the user asks for prior work, a past
decision matters, or avoiding rework needs information you lack. Prefer the
smallest retrieval: recent context or one targeted search. Fetch a full
observation only when its preview is insufficient. Route to the relevant
project when known. A simple question may need zero memory reads.

### Save outcomes, not an activity log

Save confirmed, future-useful outcomes: significant decisions, preferences,
non-obvious discoveries, operational state, and verified root causes or fixes.
Group related changes into one observation per semantic owner, after the outcome
is established. Reuse a stable `topic_key` or `mem_update` a known observation
for an evolving subject. Do not save routine tests, listings, progress updates,
failed experiments, speculative diagnoses, or facts cheaply recoverable from
code. Do not force a save every turn.

### Safety, routing, checkpoints

Never store credentials, cookies, tokens, account identifiers, raw captured
traffic, or secret fragments. Choose the project by semantic owner, not cwd.
Route personal Codex/Engram/cross-project preferences to `codex-config`, scope
`personal`. Reuse only the authoritative registered session identity; never
invent one.

Memory operations are internal bookkeeping, never the user-facing answer.
Complete required memory work before composing the completed-task reply;
send the complete answer as the final message of the turn with no later tool calls.
If memory work fails or needs follow-up, still send the answer.

Summarize only at a genuine session close or compaction, not after a normal
reply, and never twice for one boundary. On a `COMPACTION CHECKPOINT` marker,
save one `mem_session_summary` of the compacted work, then continue.
