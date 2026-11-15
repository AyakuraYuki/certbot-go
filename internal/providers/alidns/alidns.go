package alidns

import (
	"errors"
	"fmt"

	aliDNS "github.com/alibabacloud-go/alidns-20150109/v5/client"
	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	"github.com/aliyun/credentials-go/credentials"
	"github.com/go-acme/lego/v4/challenge/dns01"

	"github.com/AyakuraYuki/certbot-go/internal/config"
	"github.com/AyakuraYuki/certbot-go/internal/log"
	"github.com/AyakuraYuki/certbot-go/internal/providers/resolver"
)

type Provider struct {
	conf   *openapi.Config
	client *aliDNS.Client

	resolver *resolver.Resolver
}

func NewProvider(conf *config.Config, delegations map[string]string) (*Provider, error) {
	provider := &Provider{
		resolver: resolver.New(delegations),
	}

	credConfig := new(credentials.Config).
		SetType("access_key").
		SetAccessKeyId(conf.AliDNS.AccessKeyID).
		SetAccessKeySecret(conf.AliDNS.AccessKeySecret)

	akCred, err := credentials.NewCredential(credConfig)
	if err != nil {
		return nil, fmt.Errorf("[alidns] create credential: %w", err)
	}

	provider.conf = new(openapi.Config).
		SetCredential(akCred).
		SetEndpoint("alidns.aliyuncs.com")

	provider.client, err = aliDNS.NewClient(provider.conf)
	if err != nil {
		return nil, fmt.Errorf("[alidns] create client: %w", err)
	}

	return provider, nil
}

// Present creates a TXT record for the DNS-01 challenge.
func (p *Provider) Present(domain, token, keyAuth string) error {
	info := dns01.GetChallengeInfo(domain, keyAuth)

	rr, domainName, err := p.resolver.Resolve(domain)
	if err != nil {
		return fmt.Errorf("cannot resolve record %q, error: %w", domain, err)
	}

	log.Info().Msgf("[alidns] Adding TXT record: %s.%s = %s", rr, domainName, info.Value)

	return p.addRecord(domainName, rr, "TXT", info.Value, 600)
}

// CleanUp removes the TXT record after challenge validation.
func (p *Provider) CleanUp(domain, token, keyAuth string) error {
	info := dns01.GetChallengeInfo(domain, keyAuth)

	rr, domainName, err := p.resolver.Resolve(domain)
	if err != nil {
		return fmt.Errorf("cannot resolve record %q, error: %w", domain, err)
	}

	subDomain := rr + "." + domainName
	log.Info().Msgf("[alidns] Cleaning up TXT record: %s", subDomain)

	records, err := p.findSubDomainRecords(subDomain, "TXT")
	if err != nil {
		return fmt.Errorf("cannot find TXT records for %q, error: %w", subDomain, err)
	}

	for _, rec := range records {
		if *rec.Value == info.Value {
			log.Info().Msgf("[alidns] Deleting record ID: %s", *rec.RecordId)
			if err = p.deleteRecord(*rec.RecordId); err != nil {
				return fmt.Errorf("cannot delete record %q, error: %w", *rec.RecordId, err)
			}
		}
	}

	return nil
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
