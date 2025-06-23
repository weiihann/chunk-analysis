package logger

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
)

type LogLevel string

const (
	LevelDebug LogLevel = "debug"
	LevelInfo  LogLevel = "info"
	LevelWarn  LogLevel = "warn"
	LevelError LogLevel = "error"
)

type Config struct {
	Level        LogLevel
	Format       string // "text" or "json"
	EnableColors bool
}

var defaultLogger *slog.Logger

// ANSI color codes
const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorYellow = "\033[33m"
	ColorBlue   = "\033[34m"
	ColorGray   = "\033[37m"
	ColorBold   = "\033[1m"
)

// ColoredTextHandler wraps slog.TextHandler to add colors
type ColoredTextHandler struct {
	*slog.TextHandler
	enableColors bool
}

func NewColoredTextHandler(w io.Writer, opts *slog.HandlerOptions, enableColors bool) *ColoredTextHandler {
	return &ColoredTextHandler{
		TextHandler:  slog.NewTextHandler(w, opts),
		enableColors: enableColors,
	}
}

func (h *ColoredTextHandler) Handle(ctx context.Context, r slog.Record) error {
	if !h.enableColors {
		return h.TextHandler.Handle(ctx, r)
	}

	// Create a new record with colored level
	newRecord := slog.NewRecord(r.Time, r.Level, h.colorizeMessage(r.Level, r.Message), r.PC)

	// Copy all attributes
	r.Attrs(func(a slog.Attr) bool {
		newRecord.AddAttrs(a)
		return true
	})

	return h.TextHandler.Handle(ctx, newRecord)
}

func (h *ColoredTextHandler) colorizeMessage(level slog.Level, message string) string {
	if !h.enableColors {
		return message
	}

	var color string
	switch level {
	case slog.LevelDebug:
		color = ColorGray
	case slog.LevelInfo:
		color = ColorBlue
	case slog.LevelWarn:
		color = ColorYellow
	case slog.LevelError:
		color = ColorRed
	default:
		return message
	}

	return color + ColorBold + message + ColorReset
}

// isTerminal checks if the output is a terminal (TTY)
func isTerminal(w io.Writer) bool {
	if f, ok := w.(*os.File); ok {
		stat, err := f.Stat()
		if err != nil {
			return false
		}
		return (stat.Mode() & os.ModeCharDevice) != 0
	}
	return false
}

// Initialize sets up the global logger with the specified configuration
func Initialize(config Config) {
	var level slog.Level
	switch strings.ToLower(string(config.Level)) {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level: level,
	}

	var handler slog.Handler
	if config.Format == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		// Auto-detect terminal support for colors if not explicitly disabled
		enableColors := config.EnableColors
		if enableColors && !isTerminal(os.Stdout) {
			enableColors = false // Disable colors if not in a terminal
		}

		handler = NewColoredTextHandler(os.Stdout, opts, enableColors)
	}

	defaultLogger = slog.New(handler)
	slog.SetDefault(defaultLogger)
}

// GetLogger returns a logger with component context
func GetLogger(component string) *slog.Logger {
	if defaultLogger == nil {
		// Initialize with default config if not already initialized
		Initialize(Config{Level: LevelInfo, Format: "text", EnableColors: true})
	}
	return defaultLogger.With("component", component)
}

// Default logger methods for convenience
func Debug(msg string, args ...any) {
	if defaultLogger == nil {
		Initialize(Config{Level: LevelInfo, Format: "text", EnableColors: true})
	}
	defaultLogger.Debug(msg, args...)
}

func Info(msg string, args ...any) {
	if defaultLogger == nil {
		Initialize(Config{Level: LevelInfo, Format: "text", EnableColors: true})
	}
	defaultLogger.Info(msg, args...)
}

func Warn(msg string, args ...any) {
	if defaultLogger == nil {
		Initialize(Config{Level: LevelInfo, Format: "text", EnableColors: true})
	}
	defaultLogger.Warn(msg, args...)
}

func Error(msg string, args ...any) {
	if defaultLogger == nil {
		Initialize(Config{Level: LevelInfo, Format: "text", EnableColors: true})
	}
	defaultLogger.Error(msg, args...)
}
