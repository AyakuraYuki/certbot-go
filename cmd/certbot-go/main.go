package main

import (
	"os"

	"github.com/AyakuraYuki/certbot-go/cli"
	"github.com/AyakuraYuki/certbot-go/internal/signal"
)

var (
	version = "dev"
)

func init() {

}

func main() {
	// Setup signal handler for dumping goroutine stack traces
	signal.SetupStackDumpSignal()

	// Set version in CLI package
	cli.SetVersion(version)

	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
