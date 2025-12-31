package logger

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// Interface -.
type Interface interface {
	Debug(message interface{}, args ...interface{})
	Info(message string, args ...interface{})
	Warn(message string, args ...interface{})
	Error(message interface{}, args ...interface{})
	Fatal(message interface{}, args ...interface{})
}

// Logger -.
type Logger struct {
	logger zerolog.Logger
}

func New(level string) *Logger {
	lvl := parseLevel(level)

	zerolog.SetGlobalLevel(lvl)
	zerolog.TimeFieldFormat = time.RFC3339

	var out io.Writer = os.Stdout

	// Для локального запуску / тестів: кольоровий “console” формат
	if strings.ToLower(os.Getenv("LOG_FORMAT")) == "console" {
		out = zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: time.RFC3339,
			NoColor:    false,
		}
	}

	log := zerolog.New(out).Level(lvl).With().Timestamp().Logger()

	return &Logger{logger: log}
}

func parseLevel(level string) zerolog.Level {
	switch strings.ToLower(level) {
	case "fatal":
		return zerolog.FatalLevel
	case "error":
		return zerolog.ErrorLevel
	case "warn", "warning":
		return zerolog.WarnLevel
	case "info":
		return zerolog.InfoLevel
	case "debug":
		return zerolog.DebugLevel
	default:
		return zerolog.InfoLevel
	}
}

func (l *Logger) Debug(message interface{}, args ...interface{}) {
	l.write(zerolog.DebugLevel, message, args...)
}
func (l *Logger) Info(message string, args ...interface{}) {
	l.write(zerolog.InfoLevel, message, args...)
}
func (l *Logger) Warn(message string, args ...interface{}) {
	l.write(zerolog.WarnLevel, message, args...)
}
func (l *Logger) Error(message interface{}, args ...interface{}) {
	l.write(zerolog.ErrorLevel, message, args...)
}
func (l *Logger) Fatal(message interface{}, args ...interface{}) {
	l.write(zerolog.FatalLevel, message, args...)
	os.Exit(1)
}

func (l *Logger) write(level zerolog.Level, message interface{}, args ...interface{}) {
	ev := l.logger.WithLevel(level)

	switch m := message.(type) {
	case error:
		ev = ev.Err(m)
		if len(args) > 0 {
			ev.Msgf(m.Error(), args...)
		} else {
			ev.Msg(m.Error())
		}
	case string:
		if len(args) > 0 {
			ev.Msgf(m, args...)
		} else {
			ev.Msg(m)
		}
	default:
		ev.Msg(fmt.Sprintf("unknown message type: %T value=%v", message, message))
	}
}
