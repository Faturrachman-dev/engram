#!/usr/bin/env bash
# Engram — SessionStart hook for Codex
#
# 1. Ensures the engram server is running
# 2. Creates a session in engram
# 3. Auto-imports git-synced chunks if .engram/manifest.json exists
# 4. Injects Memory Protocol instructions + memory context

ENGRAM_PORT="${ENGRAM_PORT:-7437}"
ENGRAM_EXTERNAL_URL="${ENGRAM_URL:-}"
ENGRAM_EXTERNAL_URL="${ENGRAM_EXTERNAL_URL#"${ENGRAM_EXTERNAL_URL%%[![:space:]]*}"}"
ENGRAM_EXTERNAL_URL="${ENGRAM_EXTERNAL_URL%"${ENGRAM_EXTERNAL_URL##*[![:space:]]}"}"
if [ -n "$ENGRAM_EXTERNAL_URL" ]; then
  ENGRAM_URL="$ENGRAM_EXTERNAL_URL"
  ENGRAM_MANAGED_LOCAL=0
else
  ENGRAM_URL="http://127.0.0.1:${ENGRAM_PORT}"
  ENGRAM_MANAGED_LOCAL=1
fi
IMPORT_TIMEOUT_SECS=8
LOCK_TTL_SECS=$((IMPORT_TIMEOUT_SECS + 4))
LOCK_METADATA_STALE_SECS=$((LOCK_TTL_SECS * 5))

# Load shared helpers
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "${SCRIPT_DIR}/_helpers.sh"

# Read hook input from stdin
INPUT=$(cat)
CWD=$(echo "$INPUT" | jq -r '.cwd // empty')

# Explicit ENGRAM_URL intentionally delegates ownership to an external server.
if [ "$ENGRAM_MANAGED_LOCAL" = 1 ]; then
  ENGRAM_INSTANCE_ID=$(engram instance-id 2>/dev/null) || {
    printf '%s\n' "warning: Engram could not resolve its local server identity." >&2
    exit 0
  }
if ! engram_health_matches_instance "$ENGRAM_INSTANCE_ID"; then
  ENGRAM_SERVE_DATA_DIR="${ENGRAM_DATA_DIR:-$HOME/.engram}"
  if mkdir -p "$ENGRAM_SERVE_DATA_DIR" 2>/dev/null && : >> "$ENGRAM_SERVE_DATA_DIR/serve.err.log" 2>/dev/null; then
    ENGRAM_SERVE_ERR_LOG="$ENGRAM_SERVE_DATA_DIR/serve.err.log"
  else
    ENGRAM_SERVE_ERR_LOG="${TMPDIR:-/tmp}/engram-serve.err.log"
  fi
  if [ "${OS:-}" = "Windows_NT" ]; then
    ENGRAM_CLOUD_AUTOSYNC=1 engram serve-background "$ENGRAM_SERVE_ERR_LOG" || true
  else
    ENGRAM_CLOUD_AUTOSYNC=1 engram serve > /dev/null 2>> "$ENGRAM_SERVE_ERR_LOG" &
  fi
  sleep 0.5
fi
if ! engram_health_matches_instance "$ENGRAM_INSTANCE_ID"; then
  printf '%s\n' "warning: Engram server ownership mismatch; use ENGRAM_URL, ENGRAM_PORT, or ENGRAM_SOCKET to isolate it." >&2
  exit 0
fi
fi

PROJECT=$(resolve_project "$CWD") || PROJECT=""

# Register and retain only the server-confirmed runtime identity.
SESSION_HANDOFF=$(engram_session_handoff "$INPUT" "$PROJECT" "$CWD" codex-register) && SESSION_REGISTERED=1 || SESSION_REGISTERED=0

# Auto-import git-synced chunks
if [ -f "${CWD}/.engram/manifest.json" ]; then
  (
    cd "$CWD" 2>/dev/null || exit 0
    IMPORT_LOCK="/tmp/engram-sync-import-$(printf '%s' "$CWD" | cksum | cut -d ' ' -f 1).lock"
    write_import_lock_info() {
      LOCK_INFO_TMP="$IMPORT_LOCK/info.$$"
      printf '%s %s\n' "$LOCK_PID" "$LOCK_NOW" > "$LOCK_INFO_TMP" 2>/dev/null \
        && mv "$LOCK_INFO_TMP" "$IMPORT_LOCK/info" 2>/dev/null
    }
    lock_path_age_secs() {
      LOCK_PATH=$1
      LOCK_MTIME=""
      if LOCK_MTIME=$(stat -f %m "$LOCK_PATH" 2>/dev/null); then
        :
      elif LOCK_MTIME=$(stat -c %Y "$LOCK_PATH" 2>/dev/null); then
        :
      else
        return 1
      fi
      case "$LOCK_MTIME" in
        ''|*[!0-9]*) return 1 ;;
      esac
      printf '%s\n' $(( LOCK_NOW - LOCK_MTIME ))
    }
    lock_metadata_is_stale() {
      LOCK_METADATA_PATH=$1
      LOCK_METADATA_AGE=$(lock_path_age_secs "$LOCK_METADATA_PATH") || return 1
      [ "$LOCK_METADATA_AGE" -gt "$LOCK_METADATA_STALE_SECS" ]
    }
    acquire_import_lock() {
      LOCK_PID="${BASHPID:-$$}"
      LOCK_NOW=$(date +%s)
      if mkdir "$IMPORT_LOCK" 2>/dev/null; then
        write_import_lock_info || true
        return 0
      fi

      LOCK_INFO=""
      if [ -f "$IMPORT_LOCK/info" ]; then
        LOCK_INFO=$(cat "$IMPORT_LOCK/info" 2>/dev/null || true)
      fi
      read -r OLD_PID OLD_EPOCH LOCK_INFO_EXTRA <<< "$LOCK_INFO"
      STALE_LOCK=0
      if [ -z "$LOCK_INFO" ] || [ -z "$OLD_PID" ] || [ -z "$OLD_EPOCH" ] || [ -n "$LOCK_INFO_EXTRA" ]; then
        lock_metadata_is_stale "$IMPORT_LOCK" || return 1
        STALE_LOCK=1
      elif [[ "$OLD_PID" == *[!0-9]* ]] || [[ "$OLD_EPOCH" == *[!0-9]* ]]; then
        lock_metadata_is_stale "$IMPORT_LOCK/info" || return 1
        STALE_LOCK=1
      elif kill -0 "$OLD_PID" 2>/dev/null; then
        return 1
      elif [ $(( LOCK_NOW - OLD_EPOCH )) -gt "$LOCK_TTL_SECS" ]; then
        STALE_LOCK=1
      fi

      if [ "$STALE_LOCK" -ne 1 ]; then
        return 1
      fi
      rm -f "$IMPORT_LOCK/info" 2>/dev/null || true
      rmdir "$IMPORT_LOCK" 2>/dev/null || return 1
      if mkdir "$IMPORT_LOCK" 2>/dev/null; then
        write_import_lock_info || true
        return 0
      fi
      return 1
    }
    if ! acquire_import_lock; then
      exit 0
    fi
    trap 'rm -f "$IMPORT_LOCK/info" 2>/dev/null || true; rmdir "$IMPORT_LOCK" 2>/dev/null || true' EXIT
    if command -v timeout >/dev/null 2>&1; then
      timeout "${IMPORT_TIMEOUT_SECS}s" engram sync --import >/dev/null 2>&1 || true
    else
      engram sync --import >/dev/null 2>&1 &
      IMPORT_PID=$!
      (sleep "$IMPORT_TIMEOUT_SECS"; kill "$IMPORT_PID" 2>/dev/null || true) &
      WAITER_PID=$!
      wait "$IMPORT_PID" 2>/dev/null || true
      kill "$WAITER_PID" 2>/dev/null || true
    fi
  ) >/dev/null 2>&1 &
fi

# Fetch memory context
CONTEXT=""
if [ -n "$PROJECT" ]; then
  ENCODED_PROJECT=$(printf '%s' "$PROJECT" | jq -sRr @uri)
  CONTEXT=$(curl -sf "${ENGRAM_URL}/context?project=${ENCODED_PROJECT}&compact=true&max_bytes=2048&sessions=2&prompts=-1&observations=3&pinned=2" --max-time 3 2>/dev/null | jq -r '.context // empty')
fi

# The persistent model instructions already contain the memory protocol.
# Do not inject it again or force a first-turn context fetch.
printf '%s\n' "$SESSION_HANDOFF"

# Inject memory context if available
if [ -n "$CONTEXT" ]; then
  printf "\n%s\n" "$CONTEXT"
fi

exit 0
