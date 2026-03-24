#!/usr/bin/env bash
# .claude/skills/lib/rest.sh
# Chat completion helpers for OpenRouter and Ollama.
# Usage: . .claude/skills/lib/rest.sh   (must be sourced, not executed)

# chat PROVIDER MODEL PAYLOAD_FILE
#   Sends a chat completion request and prints the full response JSON.
#   PROVIDER  — "openrouter" or "ollama"
#   MODEL     — model identifier string
#   PAYLOAD_FILE — path to a JSON file with the request body
#   Returns 1 on HTTP / curl error.
chat() {
  local provider="$1"
  local model="$2"
  local payload_file="$3"

  case "$provider" in
    openrouter)
      load_env_key OPENROUTER_API_KEY || return 1
      curl -s --fail-with-body \
        --connect-timeout 30 --max-time 120 \
        -X POST "https://openrouter.ai/api/v1/chat/completions" \
        -H "Authorization: Bearer ${OPENROUTER_API_KEY}" \
        -H "Content-Type: application/json" \
        -d "@${payload_file}"
      ;;
    ollama)
      local base="${OLLAMA_BASE_URL:-http://localhost:11434}"
      # Reject plain HTTP for non-loopback hosts to prevent exfiltration.
      if [[ "$base" =~ ^http:// ]] && \
         [[ ! "$base" =~ ^http://(localhost|127\.|::1) ]]; then
        echo "ERROR: OLLAMA_BASE_URL uses plain HTTP for a non-loopback host." >&2
        echo "       Use https:// or a localhost address." >&2
        return 1
      fi
      curl -s --fail-with-body \
        --connect-timeout 30 --max-time 120 \
        -X POST "${base}/api/chat" \
        -H "Content-Type: application/json" \
        -d "@${payload_file}"
      ;;
    *)
      echo "ERROR: Unknown provider '${provider}'. Use 'openrouter' or 'ollama'." >&2
      return 1
      ;;
  esac
}

# chat_content PROVIDER MODEL PAYLOAD_FILE
#   Like chat(), but returns only the message content string
#   (choices[0].message.content for OpenRouter; message.content for Ollama).
chat_content() {
  local provider="$1"
  local model="$2"
  local payload_file="$3"

  local response
  response=$(chat "$provider" "$model" "$payload_file") || return 1

  case "$provider" in
    openrouter)
      echo "$response" | jq -r '.choices[0].message.content'
      ;;
    ollama)
      echo "$response" | jq -r '.message.content'
      ;;
  esac
}
