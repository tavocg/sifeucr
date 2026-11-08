package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
)

func newLogger(dev bool, logLvl string, logFmt string) (*slog.Logger, error) {
	level, err := parseLogLevel(logLvl)
	if err != nil {
		return nil, err
	}

	opts := &slog.HandlerOptions{
		Level: level,
	}
	if dev {
		opts.AddSource = true
	}

	switch strings.ToLower(logFmt) {
	case "json":
		return slog.New(slog.NewJSONHandler(os.Stdout, opts)), nil
	case "text":
		return slog.New(slog.NewTextHandler(os.Stdout, opts)), nil
	default:
		return nil, fmt.Errorf("invalid --logfmt %q: expected text or json", logFmt)
	}
}

func parseLogLevel(logLvl string) (slog.Level, error) {
	switch strings.ToLower(logLvl) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("invalid --loglvl %q: expected debug, info, warn, or error", logLvl)
	}
}
