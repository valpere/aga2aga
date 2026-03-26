// Package gateway implements the MCP Gateway that bridges external AI agents
// (Claude Code, Codex CLI, Gemini CLI) to the Redis Streams transport via four
// MCP tools: get_task, complete_task, fail_task, and heartbeat.
package gateway
