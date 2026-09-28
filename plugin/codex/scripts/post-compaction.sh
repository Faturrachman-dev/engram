#!/usr/bin/env bash
# Engram — Post-compaction hook for Codex
#
# When compaction happens, inject Memory Protocol + context and instruct
# the agent to persist the compacted summary via mem_session_summary.

ENGRAM_PORT="${ENGRAM_PORT:-7437}"
ENGRAM_EXTERNAL_URL="${ENGRAM_URL:-}"
ENGRAM_EXTERNAL_URL="${ENGRAM_EXTERNAL_URL#"${ENGRAM_EXTERNAL_URL%%[![:space:]]*}"}"
ENGRAM_EXTERNAL_URL="${ENGRAM_EXTERNAL_URL%"${ENGRAM_EXTERNAL_URL##*[![:space:]]}"}"
ENGRAM_URL="${ENGRAM_EXTERNAL_URL:-http://127.0.0.1:${ENGRAM_PORT}}"

# Load shared helpers
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "${SCRIPT_DIR}/_helpers.sh"

# Read hook input from stdin
INPUT=$(cat)
CWD=$(echo "$INPUT" | jq -r '.cwd // empty')
PROJECT=$(resolve_project "$CWD") || PROJECT=""

# Register and retain only the server-confirmed runtime identity.
SESSION_HANDOFF=$(engram_session_handoff "$INPUT" "$PROJECT" "$CWD" codex-resolve) && REGISTERED=1 || REGISTERED=0

# Fetch context from previous sessions
CONTEXT=""
if [ -n "$PROJECT" ]; then
  ENCODED_PROJECT=$(printf '%s' "$PROJECT" | jq -sRr @uri)
  CONTEXT=$(curl -sf "${ENGRAM_URL}/context?project=${ENCODED_PROJECT}&compact=true&max_bytes=2048&sessions=2&prompts=-1&observations=3&pinned=2" --max-time 3 2>/dev/null | jq -r '.context // empty')
fi

# The model instructions already contain the memory protocol. Preserve the
# authoritative identity and the compaction handoff without repeating it.
printf '%s\n' "$SESSION_HANDOFF"
if [ "$REGISTERED" -ne 1 ]; then
  printf '%s\n' 'Registration was not confirmed after compaction. Keep the compacted summary and any retained context for the current task; do not invoke agent-attributed memory tools until the host hook re-registers the runtime session.'
  if [ -n "$CONTEXT" ]; then
    printf '\n%s\n' "$CONTEXT"
  fi
  exit 0
fi
printf "1. FIRST: Call mem_session_summary with the compacted summary above. Use project: '%s'.\n" "$PROJECT"
printf '%s\n' '2. Read the compact context below; call mem_context only if more detail is needed.'

# Inject memory context if available
if [ -n "$CONTEXT" ]; then
  printf "\n%s\n" "$CONTEXT"
fi

exit 0
