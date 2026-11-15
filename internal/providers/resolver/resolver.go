// Package resolver computes the DNS record name (RR / sub domain) and zone for
// ACME DNS-01 challenges, applying CNAME delegation mappings. It is shared by
// all DNS providers.
package resolver

import (
	"fmt"
	"strings"
)

const challengePrefix = "_acme-challenge"

type Resolver struct {
	// delegations maps base domain → CNAME delegate zone.
	// e.g. "example.com" → "example.proxy-acme.com"
	// When lego asks to present for "example.com", we create TXT at
	// _acme-challenge.example.proxy-acme.com instead.
	delegations map[string]string
}

func New(delegations map[string]string) *Resolver {
	return &Resolver{delegations: delegations}
}

// Resolve computes the RR (subdomain part) and DomainName (zone) for the
// ACME challenge, applying CNAME delegation mapping.
//
//  1. For domain with delegation
//     domain="example.com", delegate="example.proxy-acme.com"
//     → RR = "_acme-challenge.example", DomainName = "proxy-acme.com"
//
//  2. For domain in direct mode
//     domain="example.com"
//     → RR = "_acme-challenge", DomainName = "example.com"
//     domain="api.example.com"
//     → RR = "_acme-challenge.api", DomainName = "example.com"
func (r *Resolver) Resolve(domain string) (rr, domainName string, err error) {
	// Strip wildcard prefix if present
	cleanDomain := strings.TrimPrefix(domain, "*.")

	// Look for delegation: try exact match first, then parent domain
	delegateZone, delegatedKey := r.findDelegation(cleanDomain)

	if delegateZone != "" {
		// === CNAME delegation mode ===
		return resolveWithDelegation(cleanDomain, delegateZone, delegatedKey)
	}

	// === direct mode ===
	return resolveDirect(cleanDomain)
}

// findDelegation looks up the delegation map for exact or parent domain match.
// Returns the delegate zone and the matched key, or empty string if not found.
func (r *Resolver) findDelegation(cleanDomain string) (delegateZone, delegatedKey string) {
	// Try exact match
	if zone, ok := r.delegations[cleanDomain]; ok {
		return zone, cleanDomain
	}

	// Try parent domains progressively: api.example.com → example.com
	parts := strings.Split(cleanDomain, ".")
	for i := 1; i < len(parts)-1; i++ {
		parent := strings.Join(parts[i:], ".")
		if zone, ok := r.delegations[parent]; ok {
			return zone, parent
		}
	}

	return "", ""
}

// resolveWithDelegation computes RR and zone for CNAME delegation mode.
func resolveWithDelegation(cleanDomain, delegateZone, delegatedKey string) (rr, domainName string, err error) {
	// delegateZone is like "example.proxy-acme.com"
	// Split into prefix (example) and zone (proxy-acme.com)
	zoneParts := strings.Split(delegateZone, ".")
	if len(zoneParts) < 2 {
		return "", "", fmt.Errorf("invalid delegate zone: %s", delegateZone)
	}

	domainName = strings.Join(zoneParts[len(zoneParts)-2:], ".")
	prefix := ""
	if len(zoneParts) > 2 {
		prefix = strings.Join(zoneParts[:len(zoneParts)-2], ".")
	}

	// Determine the challenge RR prefix
	recordPrefix := challengePrefix
	if cleanDomain != delegatedKey && strings.HasSuffix(cleanDomain, "."+delegatedKey) {
		// cleanDomain is a subdomain of the delegated domain
		// e.g., cleanDomain="api.example.com", delegatedKey="example.com"
		// → recordPrefix="_acme-challenge.api"
		extra := strings.TrimSuffix(cleanDomain, "."+delegatedKey)
		recordPrefix = challengePrefix + "." + extra
	}

	if prefix != "" {
		rr = recordPrefix + "." + prefix
	} else {
		rr = recordPrefix
	}

	return rr, domainName, nil
}

// resolveDirect computes RR and zone for domains directly hosted on the DNS
// provider.
func resolveDirect(cleanDomain string) (rr, domainName string, err error) {
	parts := strings.Split(cleanDomain, ".")
	if len(parts) < 2 {
		return "", "", fmt.Errorf("invalid domain: %s", cleanDomain)
	}

	// Zone = registered domain (last 2 parts for TLD like .com/.net etc.)
	// For domains like .co.uk, this would need a public suffix list,
	// but for typical usage (.com, .net) last 2 parts is correct.
	domainName = strings.Join(parts[len(parts)-2:], ".")

	// RR = _acme-challenge[.subdomain-prefix]
	if len(parts) > 2 {
		// e.g., "api.example.com" → RR = "_acme-challenge.api"
		subParts := parts[:len(parts)-2]
		rr = challengePrefix + "." + strings.Join(subParts, ".")
	} else {
		// e.g., "example.com" → RR = "_acme-challenge"
		rr = challengePrefix
	}

	return rr, domainName, nil
}
