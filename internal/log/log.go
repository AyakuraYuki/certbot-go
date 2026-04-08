package log

import (
	"fmt"
	"os"
	"time"

	goacmeLog "github.com/go-acme/lego/v4/log"
	"github.com/rs/zerolog"
)

var (
	logger  zerolog.Logger
	noColor bool
)

func init() {
	noColor = !isColorSupported()

	locShanghai, _ := time.LoadLocation("Asia/Shanghai")

	output := zerolog.ConsoleWriter{
		Out:             os.Stderr,
		TimeFormat:      time.DateTime,
		TimeLocation:    locShanghai,
		NoColor:         noColor,
		FormatMessage:   formatMessage,
		FormatLevel:     formatLevel,
		FormatFieldName: formatFieldName,
	}

	logger = zerolog.New(output).With().Timestamp().Logger()

	wrapper := &loggerWrapper{logger: logger}
	goacmeLog.Logger = wrapper
}

func Debug() *zerolog.Event { return logger.Debug() }
func Info() *zerolog.Event  { return logger.Info() }
func Warn() *zerolog.Event  { return logger.Warn() }
func Error() *zerolog.Event { return logger.Error() }
func Fatal() *zerolog.Event { return logger.Fatal() }
func Panic() *zerolog.Event { return logger.Panic() }
func Trace() *zerolog.Event { return logger.Trace() }

var _ goacmeLog.StdLogger = (*loggerWrapper)(nil)

type loggerWrapper struct {
	logger zerolog.Logger
}

func (l *loggerWrapper) Fatal(args ...any) {
	e := l.logger.Fatal()
	for i, argv := range args {
		e = e.Any(fmt.Sprintf("arg%d", i), argv)
	}
	e.Send()

	os.Exit(1)
}

func (l *loggerWrapper) Fatalln(args ...any) {
	e := l.logger.Fatal()
	for i, argv := range args {
		e = e.Any(fmt.Sprintf("arg%d", i), argv)
	}
	e.Msg("\n")

	os.Exit(1)
}

func (l *loggerWrapper) Fatalf(format string, args ...any) {
	l.logger.Fatal().Msgf(format, args...)

	os.Exit(1)
}

func (l *loggerWrapper) Print(args ...any) {
	e := l.logger.Info()
	for i, argv := range args {
		e = e.Any(fmt.Sprintf("arg%d", i), argv)
	}
	e.Send()
}

func (l *loggerWrapper) Println(args ...any) {
	e := l.logger.Info()
	for i, argv := range args {
		e = e.Any(fmt.Sprintf("arg%d", i), argv)
	}
	e.Msg("\n")
}

func (l *loggerWrapper) Printf(format string, args ...any) {
	l.logger.Info().Msgf(format, args...)
}
