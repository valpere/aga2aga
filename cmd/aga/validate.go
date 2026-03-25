package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/valpere/aga2aga/pkg/document"
)

func newValidateCmd() *cobra.Command {
	var strict bool

	cmd := &cobra.Command{
		Use:   "validate <file>",
		Short: "Validate a Skills Document (all 3 layers)",
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
			errs := v.Validate(doc)
			if len(errs) == 0 && !strict {
				fmt.Fprintf(cmd.OutOrStdout(), "OK: %s\n", path)
				return nil
			}
			for _, e := range errs {
				fmt.Fprintln(cmd.ErrOrStderr(), e.Error())
			}
			if len(errs) > 0 {
				return fmt.Errorf("validation failed: %d error(s)", len(errs))
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&strict, "strict", false, "treat semantic warnings as errors")
	return cmd
}
