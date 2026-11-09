package logging

import (
	"log/slog"
	"os"

	"github.com/lmittmann/tint"
)

// InitLogger initializes the global logger with colors and source location
func InitLogger() {
	handler := tint.NewHandler(os.Stdout, &tint.Options{
		AddSource:  true,  // Include source file and line number
		Level:      slog.LevelInfo,
		TimeFormat: "15:04:05", // HH:MM:SS format
	})

	logger := slog.New(handler)
	slog.SetDefault(logger)
}

