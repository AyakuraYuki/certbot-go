package dnspod

import (
	"errors"
	"fmt"

	"github.com/go-acme/lego/v4/challenge/dns01"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	tcErrors "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/errors"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	dnspod "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/dnspod/v20210323"

	"github.com/AyakuraYuki/certbot-go/internal/config"
	"github.com/AyakuraYuki/certbot-go/internal/log"
	"github.com/AyakuraYuki/certbot-go/internal/providers/resolver"
)

const (
	// defaultRecordLine is the default resolution line of DNSPod, required when
	// creating records.
	defaultRecordLine = "默认"

	// recordTTL is the TTL of challenge records; 600 is the minimum TTL allowed
	// by the DNSPod free plan.
	recordTTL uint64 = 600

	// pageLimit is the page size used when listing records.
	pageLimit uint64 = 100
)

type Provider struct {
	client   *dnspod.Client
	resolver *resolver.Resolver
}

func NewProvider(conf *config.Config, delegations map[string]string) (*Provider, error) {
	credential := common.NewCredential(conf.DNSPod.SecretID, conf.DNSPod.SecretKey)

	cpf := profile.NewClientProfile()
	cpf.HttpProfile.Endpoint = "dnspod.tencentcloudapi.com"

	// DNSPod API is region-agnostic, region can be empty
	client, err := dnspod.NewClient(credential, conf.DNSPod.Region, cpf)
	if err != nil {
		return nil, fmt.Errorf("[dnspod] create client: %w", err)
	}

	return &Provider{
		client:   client,
		resolver: resolver.New(delegations),
	}, nil
}

// Present creates a TXT record for the DNS-01 challenge.
func (p *Provider) Present(domain, token, keyAuth string) error {
	info := dns01.GetChallengeInfo(domain, keyAuth)

	rr, domainName, err := p.resolver.Resolve(domain)
	if err != nil {
		return fmt.Errorf("cannot resolve record %q, error: %w", domain, err)
	}

	log.Info().Msgf("[dnspod] Adding TXT record: %s.%s = %s", rr, domainName, info.Value)

	return p.addRecord(domainName, rr, "TXT", info.Value)
}

// CleanUp removes the TXT record after challenge validation.
func (p *Provider) CleanUp(domain, token, keyAuth string) error {
	info := dns01.GetChallengeInfo(domain, keyAuth)

	rr, domainName, err := p.resolver.Resolve(domain)
	if err != nil {
		return fmt.Errorf("cannot resolve record %q, error: %w", domain, err)
	}

	subDomain := rr + "." + domainName
	log.Info().Msgf("[dnspod] Cleaning up TXT record: %s", subDomain)

	records, err := p.findSubDomainRecords(domainName, rr, "TXT")
	if err != nil {
		return fmt.Errorf("cannot find TXT records for %q, error: %w", subDomain, err)
	}

	for _, rec := range records {
		if rec.Value == nil || rec.RecordId == nil || *rec.Value != info.Value {
			continue
		}
		log.Info().Msgf("[dnspod] Deleting record ID: %d", *rec.RecordId)
		if err = p.deleteRecord(domainName, *rec.RecordId); err != nil {
			return fmt.Errorf("cannot delete record %d, error: %w", *rec.RecordId, err)
		}
	}

	return nil
}

func (p *Provider) addRecord(domainName, subDomain, recordType, value string) error {
	request := dnspod.NewCreateRecordRequest()
	request.Domain = common.StringPtr(domainName)
	request.SubDomain = common.StringPtr(subDomain)
	request.RecordType = common.StringPtr(recordType)
	request.RecordLine = common.StringPtr(defaultRecordLine)
	request.Value = common.StringPtr(value)
	request.TTL = common.Uint64Ptr(recordTTL)

	_, err := p.client.CreateRecord(request)
	if err != nil {
		log.Error().Err(err).Msgf("[dnspod] Failed to add domain record")
	}
	return err
}

func (p *Provider) findSubDomainRecords(domainName, subDomain, recordType string) ([]*dnspod.RecordListItem, error) {
	var records []*dnspod.RecordListItem

	for offset := uint64(0); ; offset += pageLimit {
		request := dnspod.NewDescribeRecordListRequest()
		request.Domain = common.StringPtr(domainName)
		request.Subdomain = common.StringPtr(subDomain)
		request.RecordType = common.StringPtr(recordType)
		request.Offset = common.Uint64Ptr(offset)
		request.Limit = common.Uint64Ptr(pageLimit)

		result, err := p.client.DescribeRecordList(request)
		if err != nil {
			// DNSPod returns an error instead of an empty list when no record matches
			var sdkErr *tcErrors.TencentCloudSDKError
			if errors.As(err, &sdkErr) && sdkErr.Code == dnspod.RESOURCENOTFOUND_NODATAOFRECORD {
				return records, nil
			}
			log.Error().Err(err).Msgf("[dnspod] Failed to find subdomain records")
			return nil, err
		}
		if result == nil || result.Response == nil {
			return nil, errors.New("[dnspod] Remote service returned empty response")
		}

		records = append(records, result.Response.RecordList...)

		if len(result.Response.RecordList) < int(pageLimit) {
			return records, nil
		}
	}
}

func (p *Provider) deleteRecord(domainName string, recordID uint64) error {
	request := dnspod.NewDeleteRecordRequest()
	request.Domain = common.StringPtr(domainName)
	request.RecordId = common.Uint64Ptr(recordID)

	_, err := p.client.DeleteRecord(request)
	if err != nil {
		log.Error().Err(err).Msgf("[dnspod] Failed to delete record")
	}
	return err
}
