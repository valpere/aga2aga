package admin

import (
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/valpere/aga2aga/pkg/admin"
	"github.com/valpere/aga2aga/pkg/document"
	"github.com/valpere/aga2aga/pkg/protocol"
)

type messageLogPage struct {
	Page    string
	Session sessionData
	Logs    []admin.MessageLog
	Filter  admin.MessageLogFilter
}

// handleMessageLogList renders GET /messages — the conversation log UI.
func (srv *Server) handleMessageLogList(w http.ResponseWriter, r *http.Request) {
	sd := sessionFromCtx(r)

	filter := admin.MessageLogFilter{
		AgentID:  r.URL.Query().Get("agent"),
		ToolName: r.URL.Query().Get("tool"),
	}
	if s := r.URL.Query().Get("since"); s != "" {
		if t, err := time.Parse(time.DateOnly, s); err == nil {
			filter.Since = t.UTC()
		}
	}
	if u := r.URL.Query().Get("until"); u != "" {
		if t, err := time.Parse(time.DateOnly, u); err == nil {
			filter.Until = t.UTC().Add(24 * time.Hour)
		}
	}

	logs, err := srv.store.ListMessageLogs(r.Context(), sd.OrgID, filter)
	if err != nil {
		http.Error(w, "failed to load message logs", http.StatusInternalServerError)
		return
	}

	srv.render(w, "messages.html", messageLogPage{
		Page: "messages", Session: sd, Logs: logs, Filter: filter,
	})
}

// validMsgLogDirections is the set of allowed direction values.
var validMsgLogDirections = map[string]bool{"send": true, "receive": true}

// validMsgLogTools is the set of MCP tool names that may appear in a log entry.
var validMsgLogTools = map[string]bool{
	"send_message": true, "receive_message": true,
	"get_task": true, "complete_task": true, "fail_task": true,
}

// handleAPIMessageLog is the JSON ingest endpoint used by HTTPMessageLogger.
// It accepts POST /api/v1/message-log with a JSON body matching MessageLogEntry.
// Protected by Bearer token — callers must have role operator or admin (CWE-285).
//
// POST /api/v1/message-log
// Authorization: Bearer <api-key>
// Content-Type: application/json
//
// Response: 204 No Content on success.
func (srv *Server) handleAPIMessageLog(w http.ResponseWriter, r *http.Request) {
	k := srv.apiKeyFromRequest(r)
	// SECURITY: restrict to operator/admin roles; viewer and agent keys must not
	// be able to inject log entries (CWE-285).
	if k == nil || (k.Role != admin.RoleOperator && k.Role != admin.RoleAdmin) {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, int64(document.MaxDocumentBytes)))
	if err != nil {
		http.Error(w, `{"error":"read error"}`, http.StatusBadRequest)
		return
	}

	// ingestEntry mirrors gateway.MessageLogEntry's JSON tags — keep in sync.
	var entry struct {
		EnvelopeID string `json:"EnvelopeID"`
		ThreadID   string `json:"ThreadID"`
		FromAgent  string `json:"FromAgent"`
		ToAgent    string `json:"ToAgent"`
		MsgType    string `json:"MsgType"`
		Direction  string `json:"Direction"`
		ToolName   string `json:"ToolName"`
		Body       string `json:"Body"`
	}
	if err := json.Unmarshal(body, &entry); err != nil {
		http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
		return
	}

	// SECURITY: validate caller-supplied identifiers and enum fields (CWE-20).
	if entry.FromAgent != "" && !admin.IsValidAgentID(entry.FromAgent) {
		http.Error(w, `{"error":"invalid from_agent"}`, http.StatusBadRequest)
		return
	}
	if entry.ToAgent != "" && !admin.IsValidAgentID(entry.ToAgent) {
		// "orchestrator" is a well-known non-agent target used by gateway tools.
		if entry.ToAgent != "orchestrator" {
			http.Error(w, `{"error":"invalid to_agent"}`, http.StatusBadRequest)
			return
		}
	}
	if !validMsgLogDirections[entry.Direction] {
		http.Error(w, `{"error":"invalid direction"}`, http.StatusBadRequest)
		return
	}
	if !validMsgLogTools[entry.ToolName] {
		http.Error(w, `{"error":"invalid tool_name"}`, http.StatusBadRequest)
		return
	}
	if _, ok := protocol.Lookup(protocol.MessageType(entry.MsgType)); !ok {
		http.Error(w, `{"error":"invalid msg_type"}`, http.StatusBadRequest)
		return
	}

	m := &admin.MessageLog{
		ID:         uuid.NewString(),
		OrgID:      k.OrgID,
		EnvelopeID: entry.EnvelopeID,
		ThreadID:   entry.ThreadID,
		FromAgent:  entry.FromAgent,
		ToAgent:    entry.ToAgent,
		MsgType:    entry.MsgType,
		Direction:  entry.Direction,
		ToolName:   entry.ToolName,
		// SECURITY: always compute server-side to prevent audit integrity bypass (CWE-20).
		BodySize:  len(entry.Body),
		Body:      entry.Body,
		CreatedAt: time.Now().UTC(),
	}
	if err := srv.store.AppendMessageLog(r.Context(), m); err != nil {
		http.Error(w, `{"error":"store error"}`, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
