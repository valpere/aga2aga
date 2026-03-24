//go:build tools

// Package tools pins build-time and near-term runtime dependencies
// so they are preserved in go.mod before the packages that import
// them have been written.
package tools

import (
	_ "github.com/santhosh-tekuri/jsonschema/v6"
	_ "github.com/spf13/cobra"
	_ "gopkg.in/yaml.v3"
)
