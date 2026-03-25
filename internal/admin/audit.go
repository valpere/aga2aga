package admin

import (
	"context"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/valpere/aga2aga/pkg/admin"
)

// recordAudit appends an audit event. Errors are logged but not returned —
// audit failures must never block the primary operation.
func recordAudit(ctx context.Context, store admin.AuditStore, sd sessionData,
	action, targetType, targetID, detail string) {
	e := &admin.AuditEvent{
		ID:         uuid.New().String(),
		OrgID:      sd.OrgID,
		UserID:     sd.UserID,
		Username:   sd.Username,
		Action:     action,
		TargetType: targetType,
		TargetID:   targetID,
		Detail:     detail,
		CreatedAt:  time.Now().UTC(),
	}
	if err := store.AppendAuditEvent(ctx, e); err != nil {
		log.Printf("recordAudit: failed to append event %q for org %q: %v", action, sd.OrgID, err)
	}
}
