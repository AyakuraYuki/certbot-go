package log

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

var logger zerolog.Logger

func init() {
	locShanghai, _ := time.LoadLocation("Asia/Shanghai")

	output := zerolog.ConsoleWriter{
		Out:             os.Stderr,
		TimeFormat:      time.DateTime,
		TimeLocation:    locShanghai,
		FormatLevel:     func(i any) string { return strings.ToUpper(fmt.Sprintf("| %-6s |", i)) },
		FormatFieldName: func(i any) string { return fmt.Sprintf("%s:", i) },
	}

	logger = zerolog.New(output).With().Timestamp().Logger()
}

func Debug() *zerolog.Event { return logger.Debug() }
func Info() *zerolog.Event  { return logger.Info() }
func Warn() *zerolog.Event  { return logger.Warn() }
func Error() *zerolog.Event { return logger.Error() }
func Fatal() *zerolog.Event { return logger.Fatal() }
func Panic() *zerolog.Event { return logger.Panic() }
func Trace() *zerolog.Event { return logger.Trace() }
