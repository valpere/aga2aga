#!/usr/bin/env bash
# .claude/skills/lib/env.sh
# Load API keys from .env.local into the calling shell's environment.
# Usage: . .claude/skills/lib/env.sh   (must be sourced, not executed)

# load_env_key KEY
#   Reads KEY from .env.local (if present) and exports it.
#   Produces a clear error and returns 1 if the key is missing.
load_env_key() {
  local key="$1"

  # Already set in environment — nothing to do.
  if [[ -n "${!key}" ]]; then
    return 0
  fi

  local env_file
  env_file="$(git rev-parse --show-toplevel 2>/dev/null)/.env.local"

  if [[ ! -f "$env_file" ]]; then
    echo "ERROR: .env.local not found at ${env_file}." >&2
    echo "       Copy .env.local.example and fill in your keys." >&2
    return 1
  fi

  local value
  value=$(grep -E "^${key}=" "$env_file" | head -1 | cut -d= -f2-)

  if [[ -z "$value" ]]; then
    echo "ERROR: ${key} is not set in .env.local." >&2
    echo "       Add ${key}=<your-value> to .env.local." >&2
    return 1
  fi

  export "${key}=${value}"
}
