package logger

import (
	"fmt"
	"os"
	"path"
	"runtime"
	"strings"

	"github.com/sirupsen/logrus"
)

const defaultLogFolder = "logs/logs.log"

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

	logrus.SetOutput(f)
	logrus.SetReportCaller(true)

	switch strings.ToLower(level) {
	case "panic":
		logrus.SetLevel(logrus.PanicLevel)
	case "fatal":
		logrus.SetLevel(logrus.FatalLevel)
	case "error":
		logrus.SetLevel(logrus.ErrorLevel)
	case "warn":
		logrus.SetLevel(logrus.WarnLevel)
	case "info":
		logrus.SetLevel(logrus.InfoLevel)
	case "debug":
		logrus.SetLevel(logrus.DebugLevel)
	case "trace":
		logrus.SetLevel(logrus.TraceLevel)
	default:
		logrus.SetLevel(logrus.InfoLevel)
	}

	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
		CallerPrettyfier: func(f *runtime.Frame) (string, string) {
			funcName := path.Base(f.Function)
			fileName := fmt.Sprintf("%s:%d", path.Base(f.File), f.Line)

			return funcName, fileName
		},
	})

	return nil
}

//goland:noinspection GoUnusedExportedFunction
func Debug(format string) {
	logrus.Debug(format)
}

//goland:noinspection GoUnusedExportedFunction
func Debugf(format string, args ...any) {
	logrus.Debugf(format, args...)
}

//goland:noinspection GoUnusedExportedFunction
func Info(format string) {
	logrus.Info(format)
}

//goland:noinspection GoUnusedExportedFunction
func Warn(format string) {
	logrus.Warn(format)
}

//goland:noinspection GoUnusedExportedFunction
func Warnf(format string, args ...any) {
	logrus.Warnf(format, args...)
}

//goland:noinspection GoUnusedExportedFunction
func Infof(format string, args ...any) {
	logrus.Infof(format, args...)
}

//goland:noinspection GoUnusedExportedFunction
func Error(format string) {
	logrus.Error(format)
}

//goland:noinspection GoUnusedExportedFunction
func Errorf(format string, args ...any) {
	logrus.Errorf(format, args...)
}

//goland:noinspection GoUnusedExportedFunction
func Panic(args ...any) {
	logrus.Panic(args...)
}

//goland:noinspection GoUnusedExportedFunction
func Panicf(format string, args ...any) {
	logrus.Panicf(format, args...)
}

//goland:noinspection GoUnusedExportedFunction
func Fatalf(format string, args ...any) {
	logrus.Fatalf(format, args...)
}

//goland:noinspection GoUnusedExportedFunction
func Traceln(args ...any) {
	logrus.Traceln(args...)
}
