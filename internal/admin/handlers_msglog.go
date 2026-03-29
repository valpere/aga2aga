package admin

import (
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/valpere/aga2aga/pkg/admin"
	"github.com/valpere/aga2aga/pkg/document"
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

// handleAPIMessageLog is the JSON ingest endpoint used by HTTPMessageLogger.
// It accepts POST /api/v1/message-log with a JSON body matching MessageLogEntry.
// Protected by Bearer token (any active non-revoked key is accepted).
//
// POST /api/v1/message-log
// Authorization: Bearer <api-key>
// Content-Type: application/json
//
// Response: 204 No Content on success.
func (srv *Server) handleAPIMessageLog(w http.ResponseWriter, r *http.Request) {
	k := srv.apiKeyFromRequest(r)
	if k == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, int64(document.MaxDocumentBytes)))
	if err != nil {
		http.Error(w, `{"error":"read error"}`, http.StatusBadRequest)
		return
	}

	var entry struct {
		EnvelopeID string `json:"EnvelopeID"`
		ThreadID   string `json:"ThreadID"`
		FromAgent  string `json:"FromAgent"`
		ToAgent    string `json:"ToAgent"`
		MsgType    string `json:"MsgType"`
		Direction  string `json:"Direction"`
		ToolName   string `json:"ToolName"`
		BodySize   int    `json:"BodySize"`
		Body       string `json:"Body"`
	}
	if err := json.Unmarshal(body, &entry); err != nil {
		http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
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
		BodySize:   entry.BodySize,
		Body:       entry.Body,
		CreatedAt:  time.Now().UTC(),
	}
	if err := srv.store.AppendMessageLog(r.Context(), m); err != nil {
		http.Error(w, `{"error":"store error"}`, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
