package main

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/valpere/aga2aga/pkg/document"
)

func newInspectCmd() *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:   "inspect <file>",
		Short: "Inspect an envelope document — print envelope fields",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := args[0]
			doc, err := readAndParseFile(path)
			if err != nil {
				return err
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
			case "text":
				return printInspectText(cmd, doc)
			case "json":
				return printInspectJSON(cmd, doc)
			default:
				return fmt.Errorf("unknown format %q: must be text or json", format)
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
	// Envelope fields are always at the top level; Extra (attacker-controlled,
	// see types.go security note) is nested under "extra" to match the text
	// format's separation and prevent key shadowing.
	out := map[string]any{
		"type":    string(doc.Type),
		"version": doc.Version,
	}
	if doc.ID != "" {
		out["id"] = doc.ID
	}
	if doc.From != "" {
		out["from"] = doc.From
	}
	if len(doc.To) > 0 {
		out["to"] = []string(doc.To)
	}
	if doc.ExecID != "" {
		out["exec_id"] = doc.ExecID
	}
	if doc.Status != "" {
		out["status"] = doc.Status
	}
	if doc.InReplyTo != "" {
		out["in_reply_to"] = doc.InReplyTo
	}
	if doc.ThreadID != "" {
		out["thread_id"] = doc.ThreadID
	}
	if doc.CreatedAt != "" {
		out["created_at"] = doc.CreatedAt
	}
	if len(doc.Extra) > 0 {
		out["extra"] = doc.Extra
	}
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}
