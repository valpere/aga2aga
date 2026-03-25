package admin

import "sort"

// Evaluate returns the PolicyAction that applies when source sends a message to
// target, given a set of policies. Policies are sorted by priority (highest
// first) and the first matching policy wins. If no policy matches, the default
// is PolicyActionDeny.
//
// A policy matches if:
//   - policy.SourceID equals source or is Wildcard, AND
//   - policy.TargetID equals target or is Wildcard, OR
//   - the policy is bidirectional and the roles are swapped.
func Evaluate(policies []CommunicationPolicy, source, target string) PolicyAction {
	sorted := make([]CommunicationPolicy, len(policies))
	copy(sorted, policies)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Priority > sorted[j].Priority
	})

	for _, p := range sorted {
		if matches(p, source, target) {
			return p.Action
		}
	}
	return PolicyActionDeny
}

// matches reports whether policy p applies to a message from source to target.
func matches(p CommunicationPolicy, source, target string) bool {
	srcMatch := p.SourceID == Wildcard || p.SourceID == source
	tgtMatch := p.TargetID == Wildcard || p.TargetID == target
	if srcMatch && tgtMatch {
		return true
	}
	if p.Direction == DirectionBidirectional {
		revSrc := p.SourceID == Wildcard || p.SourceID == target
		revTgt := p.TargetID == Wildcard || p.TargetID == source
		return revSrc && revTgt
	}
	return false
}
