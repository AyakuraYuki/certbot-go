package acme

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/challenge"
	"github.com/go-acme/lego/v4/lego"
	"github.com/go-acme/lego/v4/registration"

	"github.com/AyakuraYuki/certbot-go/internal/config"
	"github.com/AyakuraYuki/certbot-go/internal/log"
)

var (
	_ registration.User = (*User)(nil)
)

// User implements registration.User for lego.
type User struct {
	Email        string                 `json:"email"`
	Registration *registration.Resource `json:"registration,omitempty"`
	KeyPEM       string                 `json:"key_pem,omitempty"`
	key          crypto.PrivateKey
}

func (u *User) GetEmail() string                        { return u.Email }
func (u *User) GetRegistration() *registration.Resource { return u.Registration }
func (u *User) GetPrivateKey() crypto.PrivateKey        { return u.key }

// Manager manages ACME operations.
type Manager struct {
	cfg      *config.Config
	user     *User
	client   *lego.Client
	provider challenge.Provider
}

func NewManager(cfg *config.Config, provider challenge.Provider) (*Manager, error) {
	m := &Manager{cfg: cfg, provider: provider}

	if err := m.loadOrCreateUser(); err != nil {
		return nil, fmt.Errorf("load/create user: %w", err)
	}

	if err := m.setupClient(); err != nil {
		return nil, fmt.Errorf("setup client: %w", err)
	}

	return m, nil
}

func (m *Manager) loadOrCreateUser() error {
	accountFile := filepath.Join(m.cfg.AccountDir, m.cfg.GetAccountFilename())

	if data, err := os.ReadFile(accountFile); err == nil {
		// Load existing account
		user := &User{}
		if err = json.Unmarshal(data, user); err != nil {
			return fmt.Errorf("parse account: %w", err)
		}

		// Decode private key
		block, _ := pem.Decode([]byte(user.KeyPEM))
		if block == nil {
			return fmt.Errorf("failed to decode account key PEM")
		}

		user.key, err = x509.ParseECPrivateKey(block.Bytes)
		if err != nil {
			return fmt.Errorf("parse account key: %w", err)
		}

		m.user = user
		log.Info().Str("email", user.Email).Msg("[acme] Loaded existing account")
		return nil
	}

	// Create new account
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("generate key: %w", err)
	}

	keyDER, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		return fmt.Errorf("marshal key: %w", err)
	}

	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	m.user = &User{
		Email:  m.cfg.Email,
		KeyPEM: string(keyPEM),
		key:    privateKey,
	}

	log.Info().Str("email", m.cfg.Email).Msg("[acme] Created new account key")
	return nil
}

func (m *Manager) setupClient() error {
	legoCfg := lego.NewConfig(m.user)
	legoCfg.CADirURL = m.cfg.ACMEDirectory

	client, err := lego.NewClient(legoCfg)
	if err != nil {
		return fmt.Errorf("create lego client: %w", err)
	}

	if err = client.Challenge.SetDNS01Provider(m.provider); err != nil {
		return fmt.Errorf("set DNS provider: %w", err)
	}

	m.client = client

	// Register if not already registered
	if m.user.Registration == nil {
		reg, err := m.client.Registration.Register(registration.RegisterOptions{
			TermsOfServiceAgreed: true,
		})
		if err != nil {
			return fmt.Errorf("register: %w", err)
		}
		m.user.Registration = reg
		log.Info().Str("email", m.user.Email).Msg("[acme] Account registered")

		if err := m.saveUser(); err != nil {
			return fmt.Errorf("save user: %w", err)
		}
	}

	return nil
}

func (m *Manager) saveUser() error {
	if err := os.MkdirAll(m.cfg.AccountDir, 0700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(m.user, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(m.cfg.AccountDir, m.cfg.GetAccountFilename()), data, 0600)
}

// ObtainOrRenew checks each certificate config and obtains/renews as needed.
func (m *Manager) ObtainOrRenew() {
	for _, certCfg := range m.cfg.Certificates {
		log.Info().Str("cert_name", certCfg.Name).Strs("domains", certCfg.Domains).Msg("[cert] Processing")

		certDir := filepath.Join(m.cfg.CertDir, certCfg.Name)

		if m.needsRenewal(certDir) {
			if err := m.obtainCert(certCfg, certDir); err != nil {
				log.Error().Err(err).Str("cert_name", certCfg.Name).Msg("[cert] ERROR obtaining/renewing")
			} else {
				log.Info().Str("cert_name", certCfg.Name).Msgf("[cert] SUCCESS: certificate saved to %s", certDir)
			}
		} else {
			log.Info().Str("cert_name", certCfg.Name).Msg("[cert] certificate is still valid, skipping")
		}
	}
}

func (m *Manager) needsRenewal(certDir string) bool {
	certFile := filepath.Join(certDir, "fullchain.pem")
	data, err := os.ReadFile(certFile)
	if err != nil {
		return true // cert doesn't exist
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return true
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return true
	}

	renewBefore := m.cfg.GetRenewBefore()
	deadline := cert.NotAfter.Add(-renewBefore)

	if time.Now().After(deadline) {
		log.Info().Msgf("[cert] Certificate expires at %s, renewal deadline %s — needs renewal", cert.NotAfter.Format(time.RFC3339), deadline.Format(time.RFC3339))
		return true
	}

	daysLeft := time.Until(cert.NotAfter).Hours() / 24
	log.Info().Msgf("[cert] Certificate valid until %s (%.0f days left)", cert.NotAfter.Format(time.RFC3339), daysLeft)
	return false
}

func (m *Manager) obtainCert(certCfg config.CertConfig, certDir string) error {
	request := certificate.ObtainRequest{
		Domains: certCfg.Domains,
		Bundle:  true,
	}

	certificates, err := m.client.Certificate.Obtain(request)
	if err != nil {
		return fmt.Errorf("obtain certificate: %w", err)
	}

	// Save certificates
	if err := os.MkdirAll(certDir, 0755); err != nil {
		return fmt.Errorf("create cert dir: %w", err)
	}

	files := map[string][]byte{
		"fullchain.pem": certificates.Certificate,
		"privkey.pem":   certificates.PrivateKey,
		"chain.pem":     certificates.IssuerCertificate,
	}

	for name, content := range files {
		if len(content) == 0 {
			continue
		}
		path := filepath.Join(certDir, name)
		if err = os.WriteFile(path, content, 0600); err != nil {
			return fmt.Errorf("write %s: %w", name, err)
		}
	}

	// Also save the metadata for potential future use
	meta, _ := json.MarshalIndent(map[string]any{
		"domains":     certCfg.Domains,
		"obtained_at": time.Now().Format(time.RFC3339),
		"cert_url":    certificates.CertURL,
		"cert_stable": certificates.CertStableURL,
	}, "", "  ")
	_ = os.WriteFile(filepath.Join(certDir, "metadata.json"), meta, 0644)

	return nil
}
