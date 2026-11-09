package logger

import (
	"larsa-academy-microservice/pkg/util"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Init initializes the zerolog logger
func Init() {
	mode := util.GetEnv("ENVIRONMENT", "")
	// Set global log level to Info
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	if strings.ToLower(mode) == "development" || strings.ToLower(mode) == "dev" {
		output := zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: time.RFC3339,
		}
		log.Logger = zerolog.New(output).With().Timestamp().Logger()
	} else {
		log.Logger = zerolog.New(os.Stderr).With().Timestamp().Logger()
	}

}
