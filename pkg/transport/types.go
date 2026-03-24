// Package transport defines the pluggable transport abstraction
// for the aga2aga message bus. Concrete implementations (Redis Streams,
// Gossip P2P) are added in later phases.
//
// pkg/document and pkg/protocol must never import this package.
package transport
