#!/usr/bin/env bash
# Engram — Post-compaction hook for Claude Code
#
# When compaction happens, inject a compact-policy pointer, one compaction
# checkpoint (persist the compacted summary once), and bounded context.

# Load shared helpers
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "${SCRIPT_DIR}/_helpers.sh"

# Read hook input from stdin
INPUT=$(cat)
SESSION_ID=$(echo "$INPUT" | jq -r '.session_id // empty')
CWD=$(echo "$INPUT" | jq -r '.cwd // empty')
PROJECT=$(resolve_project "$CWD") || PROJECT=""

# Ensure session exists
if [ -n "$SESSION_ID" ] && [ -n "$PROJECT" ]; then
  engram_curl -sf "${ENGRAM_URL}/sessions" \
    -X POST \
    -H "Content-Type: application/json" \
    -d "$(jq -n --arg id "$SESSION_ID" --arg project "$PROJECT" --arg dir "$CWD" \
      '{id: $id, project: $project, directory: $dir}')" \
    > /dev/null
fi

# Fetch context from previous sessions
CONTEXT=""
if [ -n "$PROJECT" ]; then
  ENCODED_PROJECT=$(printf '%s' "$PROJECT" | jq -sRr @uri)
  # Same bounded request as session-start.sh: compact bullets (titles kept,
  # 300-char body previews dropped), a pinned-section ceiling, and a 16 KiB total cap.
  CONTEXT=$(engram_curl -sf "${ENGRAM_URL}/context?project=${ENCODED_PROJECT}&compact=1&pinned=20&max_bytes=16384" --max-time 3 | jq -r '.context // empty' 2>/dev/null)
fi

# Resolve protocol verbosity mode for this slug. All slim/full branching
# (including the engram-version floor check) lives in Go — see `engram
# protocol-mode`. A missing/old engram binary or an unrecognized subcommand
# never yields "slim" here, so this always defaults safely to full. $mode is
# NEVER echoed/logged to this hook's own stdout.
mode=$(engram protocol-mode claude-code 2>/dev/null)
if [ "$mode" != "slim" ]; then
  mode="full"
fi

# Inject the compact-policy pointer (gated on $mode) and one unconditional
# compaction checkpoint. The checkpoint is the compaction-recovery contract;
# context is injected below, so no follow-up mem_context call is requested.
if [ "$mode" != "slim" ]; then
cat <<'PROTOCOL'
## Engram memory

Engram memory tools are available. Follow the engram-memory skill (compact
policy): work first, retrieve only when it adds missing information, save
confirmed outcomes once per semantic owner, and summarize only at a genuine
session close or compaction. Use the context below instead of refetching it.

Memory operations are internal bookkeeping, never the user-facing answer. Complete required memory work before composing the completed-task reply; send the complete answer as the final message of the turn with no later tool calls. If memory work fails or needs follow-up, still send the answer.

PROTOCOL
fi

printf "COMPACTION CHECKPOINT: save the compacted summary above once with mem_session_summary. Use project: '%s'.\n" "$PROJECT"
printf '%s\n' 'Then continue; recent context is below, so call mem_context only if more detail is needed.'

# Inject memory context if available
if [ -n "$CONTEXT" ]; then
  printf "\n%s\n" "$CONTEXT"
fi

exit 0
