package logger

import (
	"log/slog"
	"os"
	"strings"
)

// Init configures the process-wide default slog logger: JSON to stdout at
// the given level (debug/info/warn/error, case-insensitive; defaults to
// info for an empty or unrecognized value). Call once, before anything
// else logs.
func Init(levelStr string) {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: parseLevel(levelStr)})
	slog.SetDefault(slog.New(handler))
}

func parseLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
