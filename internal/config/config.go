package config

import (
	"fmt"
	"os"
	"time"

	"github.com/go-acme/lego/v4/lego"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Email         string       `yaml:"email"`
	ACMEDirectory string       `yaml:"acme_directory"`
	CertDir       string       `yaml:"cert_dir"`
	AccountDir    string       `yaml:"account_dir"`
	CheckInterval string       `yaml:"check_interval"`
	RenewBefore   string       `yaml:"renew_before"`
	AliDNS        AliDNSConfig `yaml:"alidns"`
	Certificates  []CertConfig `yaml:"certificates"`
}

type AliDNSConfig struct {
	AccessKeyID     string `yaml:"access_key_id"`
	AccessKeySecret string `yaml:"access_key_secret"`
	RegionID        string `yaml:"region_id"`
}

type CertConfig struct {
	Name              string   `yaml:"name"`
	Domains           []string `yaml:"domains"`
	ChallengeDelegate string   `yaml:"challenge_delegate"`
}

func (c *Config) GetCheckInterval() time.Duration {
	d, err := time.ParseDuration(c.CheckInterval)
	if err != nil {
		return 12 * time.Hour
	}
	return d
}

func (c *Config) GetRenewBefore() time.Duration {
	d, err := time.ParseDuration(c.RenewBefore)
	if err != nil {
		return 30 * 24 * time.Hour // 30 days
	}
	return d
}

func (c *Config) GetAccountFilename() string {
	if c.ACMEDirectory == lego.LEDirectoryProduction {
		return "account.json"
	}
	return "account_staging.json"
}

func (c *Config) Validate() error {
	if c.Email == "" {
		return fmt.Errorf("email is required")
	}
	if c.AliDNS.AccessKeyID == "" || c.AliDNS.AccessKeySecret == "" {
		return fmt.Errorf("alidns access_key_id and access_key_secret are required")
	}
	if len(c.Certificates) == 0 {
		return fmt.Errorf("at least one certificate entry is required")
	}
	for i, cert := range c.Certificates {
		if cert.Name == "" {
			return fmt.Errorf("certificates[%d].name is required", i)
		}
		if len(cert.Domains) == 0 {
			return fmt.Errorf("certificates[%d].domains must not be empty", i)
		}
	}
	return nil
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	cfg := &Config{
		// https://letsencrypt.org/getting-started/
		ACMEDirectory: lego.LEDirectoryProduction,
		CertDir:       "/etc/letsencrypt/live",
		AccountDir:    "/etc/letsencrypt/accounts",
		CheckInterval: "12h",
		RenewBefore:   "720h",
	}

	if err = yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	// expand env vars in sensitive fields
	cfg.AliDNS.AccessKeyID = os.ExpandEnv(cfg.AliDNS.AccessKeyID)
	cfg.AliDNS.AccessKeySecret = os.ExpandEnv(cfg.AliDNS.AccessKeySecret)

	if err = cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation: %w", err)
	}

	return cfg, nil
}
