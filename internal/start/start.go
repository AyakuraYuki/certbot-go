package start

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/AyakuraYuki/certbot-go/internal/acme"
	"github.com/AyakuraYuki/certbot-go/internal/config"
	"github.com/AyakuraYuki/certbot-go/internal/log"
	"github.com/AyakuraYuki/certbot-go/internal/providers/alidns"
)

func Start(configFile string, once bool) error {
	conf, err := config.LoadConfig(configFile)
	if err != nil {
		log.Error().Err(err).Str("file", configFile).Msg("Failed to load config")
		return err
	}

	log.Info().Msgf("Config loaded, ACME directory: %s", conf.ACMEDirectory)
	for _, cert := range conf.Certificates {
		mode := "direct mode"
		if cert.ChallengeDelegate != "" {
			mode = fmt.Sprintf("delegation mode (%s)", cert.ChallengeDelegate)
		}
		log.Info().Msgf("  - %s at %s", cert.Name, mode)
		for _, domain := range cert.Domains {
			log.Info().Msgf("    - %s", domain)
		}
	}

	// Build CNAME delegation map from all certificate configs
	delegations := make(map[string]string)
	for _, cert := range conf.Certificates {
		if cert.ChallengeDelegate != "" {
			for _, domain := range cert.Domains {
				// Strip wildcard for mapping
				d := domain
				if len(d) > 2 && d[:2] == "*." {
					d = d[2:]
				}
				delegations[d] = cert.ChallengeDelegate
			}
		}
	}
	log.Info().Msg("CNAME delegation mappings:")
	for domain, delegate := range delegations {
		log.Info().Msgf("  _acme-challenge.%s → _acme-challenge.%s", domain, delegate)
	}

	// Create DNS provider
	provider := alidns.NewProvider(conf, delegations)

	// Create ACME manager
	manager, err := acme.NewManager(conf, provider)
	if err != nil {
		log.Error().Err(err).Msg("Failed to initialize ACME manager")
		return err
	}

	// Run certificate check
	manager.ObtainOrRenew()

	if once {
		log.Info().Msg("Single run complete, exiting.")
		return nil
	}

	// Daemon mode: schedule periodic checks
	interval := conf.GetCheckInterval()
	log.Info().Dur("interval", interval).Msg("Entering daemon mode")

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ticker.C:
				log.Info().Msg("Running scheduled certificate check...")
				manager.ObtainOrRenew()
			case <-ctx.Done():
				log.Info().Msg("Daemon goroutine exiting.")
				return
			}
		}
	}()

	sig := <-sigCh
	cancel()
	log.Info().Msgf("Received signal %v, shutting down...", sig)

	return nil
}
