package alidns

import (
	"errors"
	"fmt"
	"strings"

	aliDNS "github.com/alibabacloud-go/alidns-20150109/v5/client"
	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	"github.com/aliyun/credentials-go/credentials"
	"github.com/go-acme/lego/v4/challenge/dns01"

	"github.com/AyakuraYuki/certbot-go/internal/config"
	"github.com/AyakuraYuki/certbot-go/internal/log"
)

const challengePrefix = "_acme-challenge"

type Provider struct {
	conf   *openapi.Config
	client *aliDNS.Client

	// delegations maps base domain → CNAME delegate zone.
	// e.g. "example.com" → "example.proxy-acme.com"
	// When lego asks to present for "example.com", we create TXT at
	// _acme-challenge.example.proxy-acme.com instead.
	delegations map[string]string
}

func NewProvider(conf *config.Config, delegations map[string]string) *Provider {
	provider := &Provider{
		client:      &aliDNS.Client{},
		delegations: delegations,
	}

	credConfig := new(credentials.Config).
		SetType("access_key").
		SetAccessKeyId(conf.AliDNS.AccessKeyID).
		SetAccessKeySecret(conf.AliDNS.AccessKeySecret)

	akCred, err := credentials.NewCredential(credConfig)
	if err != nil {
		panic(err)
	}

	provider.conf = new(openapi.Config).
		SetCredential(akCred).
		SetEndpoint("alidns.aliyuncs.com")

	provider.client, err = aliDNS.NewClient(provider.conf)
	if err != nil {
		panic(err)
	}

	return provider
}

// Present creates a TXT record for the DNS-01 challenge.
func (p *Provider) Present(domain, token, keyAuth string) error {
	info := dns01.GetChallengeInfo(domain, keyAuth)

	rr, domainName, err := p.resolveRecord(domain)
	if err != nil {
		return fmt.Errorf("cannot resolve record %q, error: %w", domain, err)
	}

	log.Info().Msgf("[dns] Adding TXT record: %s.%s = %s", rr, domainName, info.Value)

	return p.addRecord(domainName, rr, "TXT", info.Value, 600)
}

// CleanUp removes the TXT record after challenge validation.
func (p *Provider) CleanUp(domain, token, keyAuth string) error {
	info := dns01.GetChallengeInfo(domain, keyAuth)

	rr, domainName, err := p.resolveRecord(domain)
	if err != nil {
		return fmt.Errorf("cannot resolve record %q, error: %w", domain, err)
	}

	subDomain := rr + "." + domainName
	log.Info().Msgf("[dns] Cleaning up TXT record: %s", subDomain)

	records, err := p.findSubDomainRecords(subDomain, "TXT")
	if err != nil {
		return fmt.Errorf("cannot find TXT records for %q, error: %w", subDomain, err)
	}

	for _, rec := range records {
		if *rec.Value == info.Value {
			log.Info().Msgf("[dns] Deleting record ID: %s", *rec.RecordId)
			if err = p.deleteRecord(*rec.RecordId); err != nil {
				return fmt.Errorf("cannot delete record %q, error: %w", *rec.RecordId, err)
			}
		}
	}

	return nil
}

// resolveRecord computes the RR (subdomain part) and DomainName (zone) for the
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
func (p *Provider) resolveRecord(domain string) (rr, domainName string, err error) {
	// Strip wildcard prefix if present
	cleanDomain := strings.TrimPrefix(domain, "*.")

	// Look for delegation: try exact match first, then parent domain
	delegateZone, delegatedKey := p.findDelegation(cleanDomain)

	if delegateZone != "" {
		// === CNAME delegation mode ===
		return p.resolveWithDelegation(cleanDomain, delegateZone, delegatedKey)
	}

	// === direct mode ===
	return p.resolveDirect(cleanDomain)
}

// findDelegation looks up the delegation map for exact or parent domain match.
// Returns the delegate zone and the matched key, or empty string if not found.
func (p *Provider) findDelegation(cleanDomain string) (delegateZone, delegatedKey string) {
	// Try exact match
	if zone, ok := p.delegations[cleanDomain]; ok {
		return zone, cleanDomain
	}

	// Try parent domains progressively: api.example.com → example.com
	parts := strings.Split(cleanDomain, ".")
	for i := 1; i < len(parts)-1; i++ {
		parent := strings.Join(parts[i:], ".")
		if zone, ok := p.delegations[parent]; ok {
			return zone, parent
		}
	}

	return "", ""
}

// resolveWithDelegation computes RR and zone for CNAME delegation mode.
func (p *Provider) resolveWithDelegation(cleanDomain, delegateZone, delegatedKey string) (rr, domainName string, err error) {
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
		recordPrefix = challengePrefix + extra
	}

	if prefix != "" {
		rr = recordPrefix + "." + prefix
	} else {
		rr = recordPrefix
	}

	return rr, domainName, nil
}

// resolveDirect computes RR and zone for domains directly hosted on Alibaba
// Cloud DNS.
func (p *Provider) resolveDirect(cleanDomain string) (rr, domainName string, err error) {
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

func (p *Provider) addRecord(domainName, rr, recordType, value string, ttl int) error {
	_, err := p.client.AddDomainRecord(&aliDNS.AddDomainRecordRequest{
		DomainName: &domainName,
		RR:         &rr,
		Type:       &recordType,
		Value:      &value,
		TTL:        new(int64(ttl)),
	})
	if err != nil {
		log.Error().Err(err).Msgf("[alidns] Failed to add domain record")
	}
	return err
}

func (p *Provider) findSubDomainRecords(subDomain, recordType string) ([]*aliDNS.DescribeSubDomainRecordsResponseBodyDomainRecordsRecord, error) {
	result, err := p.client.DescribeSubDomainRecords(&aliDNS.DescribeSubDomainRecordsRequest{
		SubDomain: &subDomain,
		Type:      &recordType,
	})
	if err != nil {
		log.Error().Err(err).Msgf("[alidns] Failed to find subdomain records")
		return nil, err
	}
	if result == nil || result.Body == nil || result.Body.DomainRecords == nil {
		return nil, errors.New("[alidns] Remote service returned empty response")
	}
	return result.Body.DomainRecords.Record, nil
}

func (p *Provider) deleteRecord(recordID string) error {
	_, err := p.client.DeleteDomainRecord(&aliDNS.DeleteDomainRecordRequest{
		RecordId: &recordID,
	})
	if err != nil {
		log.Error().Err(err).Msgf("[alidns] Failed to delete record")
	}
	return err
}
