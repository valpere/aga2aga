// Package document implements the Skills Document engine:
// parsing, validation, building, and lifecycle management
// for the aga2aga wire-format documents.
//
// A Skills Document is a Markdown file with a YAML front-matter
// envelope that carries routing, type, and identity metadata.
// The body is human-readable Markdown passed directly to the agent.
package document
