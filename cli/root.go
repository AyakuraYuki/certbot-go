package cli

import (
	"fmt"
	"runtime/debug"
	"strings"
	"time"

	"github.com/fatih/color"
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

	goVersion string
	revision  string
	builtTime string
	modified  bool
)

func init() {
	info, ok := debug.ReadBuildInfo()
	if ok {
		goVersion = info.GoVersion
		for _, setting := range info.Settings {
			switch setting.Key {
			case "vcs.revision":
				revision = setting.Value
			case "vcs.time":
				parsedTime, _ := time.Parse(time.RFC3339, setting.Value)
				builtTime = parsedTime.Format(time.RFC1123)
			case "vcs.modified":
				modified = setting.Value == "true"
			}
		}
	}

	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "", "path to the configuration file")
	rootCmd.PersistentFlags().BoolVarP(&once, "once", "o", false, "run in once or daemon mode")
}

// SetVersion sets the version from main package
func SetVersion(version string) {
	rootCmd.Version = version

	if goVersion != "" {
		modifiedText := color.HiGreenString("Clean")
		if modified {
			modifiedText = color.HiRedString("Dirty")
		}

		rootCmd.Version = fmt.Sprintf(`%s

 Go version: %s
   Revision: %s
   Built at: %s
Dirty build: %s`,
			version, strings.TrimPrefix(goVersion, "go"), revision, builtTime, modifiedText)
	}
}

// Execute the CLI
func Execute() error {
	return rootCmd.Execute()
}
