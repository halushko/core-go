package logger

import (
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
)

const defaultLogFolder = "./logs"

//goland:noinspection GoUnusedExportedFunction
func Init() error {
	level := os.Getenv("LOG_LEVEL")
	file := os.Getenv("LOG_FILE")
	if file == "" {
		file = defaultLogFolder
	}
	f, err := os.OpenFile(file, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}

	log.SetOutput(f)

	switch strings.ToLower(level) {
	case "debug":
		log.SetLevel(log.DebugLevel)
	case "info":
		log.SetLevel(log.InfoLevel)
	case "error":
		log.SetLevel(log.ErrorLevel)
	default:
		log.SetLevel(log.InfoLevel)
	}

	log.SetFormatter(&log.TextFormatter{
		FullTimestamp: true,
	})

	return nil
}
