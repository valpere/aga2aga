package main

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/valpere/aga2aga/pkg/document"
)

func newValidateCmd() *cobra.Command {
	var strict bool

	cmd := &cobra.Command{
		Use:   "validate <file>",
		Short: "Validate an envelope document (all 3 layers)",
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
			errs := v.Validate(doc)
			var fatal []document.ValidationError
			for _, e := range errs {
				fmt.Fprintln(cmd.ErrOrStderr(), e.Error())
				if strict || e.Layer != document.LayerSemantic {
					fatal = append(fatal, e)
				}
			}
			if len(fatal) > 0 {
				return fmt.Errorf("validation failed: %d error(s)", len(fatal))
			}
			fmt.Fprintf(cmd.OutOrStdout(), "OK: %s\n", filepath.Base(path))
			return nil
		},
	}
	cmd.Flags().BoolVar(&strict, "strict", false, "treat semantic warnings as errors")
	return cmd
}
