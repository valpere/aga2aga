package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

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
			f, err := os.Open(path)
			if err != nil {
				return fmt.Errorf("open %q: %w", filepath.Base(path), err)
			}
			defer f.Close()
			raw, err := io.ReadAll(io.LimitReader(f, document.MaxDocumentBytes+1))
			if err != nil {
				return fmt.Errorf("read %q: %w", filepath.Base(path), err)
			}
			if len(raw) > document.MaxDocumentBytes {
				return fmt.Errorf("document exceeds maximum size (%d bytes)", document.MaxDocumentBytes)
			}
			doc, err := document.Parse(raw)
			if err != nil {
				return fmt.Errorf("parse %q: %w", filepath.Base(path), err)
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
			fmt.Fprintf(cmd.OutOrStdout(), "OK: %s\n", path)
			return nil
		},
	}
	cmd.Flags().BoolVar(&strict, "strict", false, "treat semantic warnings as errors")
	return cmd
}
