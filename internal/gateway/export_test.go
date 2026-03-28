package gateway

import "context"

// AuthenticateAgentForTest exposes the unexported authenticateAgent method
// for external (package gateway_test) integration tests only.
// This file is compiled exclusively during go test — never in production builds.
func (g *Gateway) AuthenticateAgentForTest(ctx context.Context, claimedAgent, rawKey string) error {
	return g.authenticateAgent(ctx, claimedAgent, rawKey)
}
