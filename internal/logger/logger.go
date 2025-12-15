// Package logger provides a configured zerolog instance.
package logger

import (
	"io"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/woozymasta/metricz-exporter/internal/service"
)

// Logger holds configuration options for the application logger.
type Logger struct {
	// Log level
	Level string `json:"level" default:"info"`

	// Log format
	Format string `json:"format" default:"text"`

	// Log output
	Output string `json:"output" default:"stderr"`
}

// Setup initializes the global logger based on provided configuration.
// It configures the output writer (File, Stdout, Stderr), format, and logging level.
func (l *Logger) Setup() {
	// fallbacks
	if l.Level == "" {
		l.Level = "info"
	}
	if l.Format == "" {
		l.Format = "text"
	}
	if l.Output == "" {
		l.Output = "stderr"
	}

	// level
	level, err := zerolog.ParseLevel(l.Level)
	if err != nil {
		level = zerolog.InfoLevel
	}

	zerolog.SetGlobalLevel(level)

	// writer
	var writer io.Writer
	switch l.Output {
	case "stdout":
		writer = os.Stdout

	case "stderr":
		writer = os.Stderr

	default:
		file, err := os.OpenFile(l.Output, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		if err != nil {
			tempLogger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).With().Timestamp().Logger()
			tempLogger.Fatal().Err(err).Str("path", l.Output).Msg("failed to open log file")
		}
		writer = file
	}

	// format
	if l.Format == "json" {
		log.Logger = zerolog.New(writer).With().Timestamp().Logger()
	} else {
		consoleWriter := zerolog.ConsoleWriter{
			Out:        writer,
			TimeFormat: time.RFC3339,
		}

		// detect color support
		if f, ok := writer.(*os.File); ok {
			consoleWriter.NoColor = !service.CanUseANSIColors(f)
		}

		log.Logger = log.Output(consoleWriter)
	}
}
