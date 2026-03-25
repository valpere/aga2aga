package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/valpere/aga2aga/pkg/document"
	"github.com/valpere/aga2aga/pkg/protocol"
)

func newCreateCmd() *cobra.Command {
	var (
		flagID     string
		flagFrom   string
		flagTo     []string
		flagExecID string
		flagFields []string
		flagOut    string
	)

	cmd := &cobra.Command{
		Use:   "create <type>",
		Short: "Create a Skills Document of the given message type",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			msgType := protocol.MessageType(args[0])
			if _, ok := protocol.Lookup(msgType); !ok {
				return fmt.Errorf("unknown message type %q; registered types: use 'aga create --help'", msgType)
			}

			b := document.NewBuilder(msgType)
			if flagID != "" {
				b = b.ID(flagID)
			}
			if flagFrom != "" {
				b = b.From(flagFrom)
			}
			if len(flagTo) > 0 {
				b = b.To(flagTo...)
			}
			if flagExecID != "" {
				b = b.ExecID(flagExecID)
			}
			for _, kv := range flagFields {
				parts := strings.SplitN(kv, "=", 2)
				if len(parts) != 2 {
					return fmt.Errorf("--field %q: expected key=value format", kv)
				}
				b = b.Field(parts[0], parts[1])
			}

			doc, err := b.Build()
			if err != nil {
				return fmt.Errorf("build: %w", err)
			}
			raw, err := document.Serialize(doc)
			if err != nil {
				return fmt.Errorf("serialize: %w", err)
			}

			if flagOut != "" {
				return os.WriteFile(flagOut, raw, 0o644)
			}
			_, err = fmt.Fprint(cmd.OutOrStdout(), string(raw))
			return err
		},
	}

	cmd.Flags().StringVar(&flagID, "id", "", "document id")
	cmd.Flags().StringVar(&flagFrom, "from", "", "sender agent id")
	cmd.Flags().StringArrayVar(&flagTo, "to", nil, "recipient(s) — repeat for multiple")
	cmd.Flags().StringVar(&flagExecID, "exec-id", "", "execution/workflow id (exec_id envelope field)")
	cmd.Flags().StringArrayVar(&flagFields, "field", nil, "extra field as key=value — repeat for multiple")
	cmd.Flags().StringVar(&flagOut, "out", "", "write output to file instead of stdout")

	return cmd
}
