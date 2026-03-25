package transport_test

import (
	"testing"
	"time"

	"github.com/valpere/aga2aga/pkg/document"
	"github.com/valpere/aga2aga/pkg/transport"
)

func TestDelivery_Fields(t *testing.T) {
	doc := &document.Document{}
	d := transport.Delivery{
		Doc:      doc,
		MsgID:    "1234-0",
		RecvedAt: time.Now(),
	}
	if d.Doc != doc {
		t.Errorf("Delivery.Doc = %v, want %v", d.Doc, doc)
	}
	if d.MsgID != "1234-0" {
		t.Errorf("Delivery.MsgID = %q, want %q", d.MsgID, "1234-0")
	}
	if d.RecvedAt.IsZero() {
		t.Error("Delivery.RecvedAt should not be zero")
	}
}
