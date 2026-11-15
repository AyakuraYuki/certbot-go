package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

const certs = `
email: a@example.com
certificates:
  - name: example.com
    domains: ["example.com"]
`

func TestLoadConfigDNSProvider(t *testing.T) {
	t.Setenv("TC_ID", "id")
	t.Setenv("TC_KEY", "key")

	cases := []struct {
		name    string
		extra   string
		want    string
		wantErr bool
	}{
		{"default alidns", "alidns: {access_key_id: a, access_key_secret: b}", DNSProviderAliDNS, false},
		{"dnspod", "dns_provider: DNSPod\ndnspod: {secret_id: $TC_ID, secret_key: $TC_KEY}", DNSProviderDNSPod, false},
		{"dnspod missing credentials", "dns_provider: dnspod", "", true},
		{"unknown provider", "dns_provider: route53", "", true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cfg, err := LoadConfig(writeConfig(t, certs+c.extra))
			if c.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if cfg.DNSProvider != c.want {
				t.Errorf("got %q, want %q", cfg.DNSProvider, c.want)
			}
			if c.want == DNSProviderDNSPod && (cfg.DNSPod.SecretID != "id" || cfg.DNSPod.SecretKey != "key") {
				t.Errorf("env vars not expanded: %+v", cfg.DNSPod)
			}
		})
	}
}
