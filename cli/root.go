package cli

import (
	"github.com/spf13/cobra"

	"github.com/AyakuraYuki/certbot-go/internal/start"
)

var rootCmd = &cobra.Command{
	Use:   "certbot-go",
	Short: `certbot-go ACME certificate manager`,
	Long:  `certbot-go is a ACME certificate manager, based on lego, provide direct and delegation mode for ACME challenge.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return start.Start(configPath, once)
	},
}

var (
	configPath string
	once       bool
)

func init() {
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "", "path to the configuration file")
	rootCmd.PersistentFlags().BoolVarP(&once, "once", "o", false, "run in once or daemon mode")
}

// SetVersion sets the version from main package
func SetVersion(version string) {
	rootCmd.Version = version
}

// Execute the CLI
func Execute() error {
	return rootCmd.Execute()
}
