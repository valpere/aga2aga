package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/valpere/aga2aga/pkg/document"
)

func newInspectCmd() *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:   "inspect <file>",
		Short: "Inspect a Skills Document — print envelope fields",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := args[0]
			raw, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("read %s: %w", path, err)
			}
			doc, err := document.Parse(raw)
			if err != nil {
				return fmt.Errorf("parse %s: %w", path, err)
			}
			v, err := document.DefaultValidator()
			if err != nil {
				return fmt.Errorf("create validator: %w", err)
			}
			if errs := v.ValidateStructural(doc); len(errs) > 0 {
				for _, e := range errs {
					fmt.Fprintln(cmd.ErrOrStderr(), e.Error())
				}
				return fmt.Errorf("inspect: %d structural error(s)", len(errs))
			}

			switch format {
			case "json":
				return printInspectJSON(cmd, doc)
			default:
				return printInspectText(cmd, doc)
			}
		},
	}
	cmd.Flags().StringVar(&format, "format", "text", "output format: text|json")
	return cmd
}

func printInspectText(cmd *cobra.Command, doc *document.Document) error {
	e := doc.Envelope
	fmt.Fprintf(cmd.OutOrStdout(), "type:     %s\n", e.Type)
	fmt.Fprintf(cmd.OutOrStdout(), "version:  %s\n", e.Version)
	if e.ID != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "id:       %s\n", e.ID)
	}
	if e.From != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "from:     %s\n", e.From)
	}
	if len(e.To) > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "to:       %s\n", e.To)
	}
	if e.ExecID != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "exec_id:  %s\n", e.ExecID)
	}
	if e.Status != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "status:   %s\n", e.Status)
	}
	if e.InReplyTo != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "in_reply_to: %s\n", e.InReplyTo)
	}
	if e.ThreadID != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "thread_id: %s\n", e.ThreadID)
	}
	if e.CreatedAt != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "created_at: %s\n", e.CreatedAt)
	}
	if len(doc.Extra) > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "extra:    %v\n", doc.Extra)
	}
	return nil
}

func printInspectJSON(cmd *cobra.Command, doc *document.Document) error {
	out := map[string]any{
		"type":    string(doc.Envelope.Type),
		"version": doc.Envelope.Version,
	}
	if doc.Envelope.ID != "" {
		out["id"] = doc.Envelope.ID
	}
	if doc.Envelope.From != "" {
		out["from"] = doc.Envelope.From
	}
	if len(doc.Envelope.To) > 0 {
		out["to"] = []string(doc.Envelope.To)
	}
	if doc.Envelope.ExecID != "" {
		out["exec_id"] = doc.Envelope.ExecID
	}
	if doc.Envelope.Status != "" {
		out["status"] = doc.Envelope.Status
	}
	if doc.Envelope.InReplyTo != "" {
		out["in_reply_to"] = doc.Envelope.InReplyTo
	}
	if doc.Envelope.ThreadID != "" {
		out["thread_id"] = doc.Envelope.ThreadID
	}
	if doc.Envelope.CreatedAt != "" {
		out["created_at"] = doc.Envelope.CreatedAt
	}
	for k, v := range doc.Extra {
		out[k] = v
	}
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}
