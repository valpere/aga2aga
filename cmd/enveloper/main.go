// Package main is the entry point for the aga2aga-enveloper CLI tool.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/valpere/aga2aga/pkg/protocol"
)

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:     "aga2aga-enveloper",
		Short:   "aga2aga-enveloper — envelope document CLI (validate, create, inspect)",
		Version: protocol.ProtocolVersion,
	}
	root.SetVersionTemplate("aga2aga-enveloper protocol version {{.Version}}\n")
	root.AddCommand(newValidateCmd())
	root.AddCommand(newCreateCmd())
	root.AddCommand(newInspectCmd())
	return root
}

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
