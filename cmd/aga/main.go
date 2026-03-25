// Package main is the entry point for the aga CLI tool.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/valpere/aga2aga/pkg/protocol"
)

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:     "aga",
		Short:   "aga — Skills Document CLI for aga2aga",
		Version: protocol.ProtocolVersion,
	}
	root.SetVersionTemplate("aga protocol version {{.Version}}\n")
	return root
}

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
