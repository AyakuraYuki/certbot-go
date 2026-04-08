package cli

import (
	"fmt"
	"runtime/debug"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/samber/lo"
	"github.com/spf13/cobra"

	"github.com/AyakuraYuki/certbot-go/internal/log"
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

	var modifiedText string
	if log.NoColor() {
		modifiedText = lo.Ternary(modified, "Dirty", "Clean")
	} else {
		modifiedText = lo.Ternary(modified, color.HiRedString("Dirty"), color.HiGreenString("Clean"))
	}

	rootCmd.Version = fmt.Sprintf(`%s

 Go version: %s
   Revision: %s
   Built at: %s
Dirty build: %s`,
		version, strings.TrimPrefix(goVersion, "go"), revision, builtTime, modifiedText)
}

// Execute the CLI
func Execute() error {
	return rootCmd.Execute()
}
