package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	defaultLogFile = "logs/logs.log"
	maxLogSize     = 10 * 1024 * 1024

	levelTrace = slog.Level(-8)
	levelFatal = slog.Level(12)
	levelPanic = slog.Level(16)
)

var active atomic.Pointer[slog.Logger]

func init() {
	active.Store(newLogger(os.Stdout, slog.LevelInfo))
}

//goland:noinspection GoUnusedExportedFunction
func Init() error {
	filename := os.Getenv("LOG_FILE")
	if filename == "" {
		filename = defaultLogFile
	}

	writer, err := newRotatingFile(filename)
	if err != nil {
		return err
	}

	active.Store(newLogger(writer, parseLevel(os.Getenv("LOG_LEVEL"))))
	return nil
}

type rotatingFile struct {
	mu       sync.Mutex
	file     *os.File
	filename string
	size     int64
}

func newRotatingFile(filename string) (*rotatingFile, error) {
	if err := os.MkdirAll(filepath.Dir(filename), 0755); err != nil {
		return nil, err
	}

	f, err := os.OpenFile(
		filename,
		os.O_CREATE|os.O_WRONLY|os.O_APPEND,
		0644,
	)
	if err != nil {
		return nil, err
	}

	info, err := f.Stat()
	if err != nil {
		err := f.Close()
		if err != nil {
			fmt.Fprintln(os.Stderr, "close file failed:", err)
		}
		return nil, err
	}

	w := &rotatingFile{
		file:     f,
		filename: filename,
		size:     info.Size(),
	}

	if w.size >= maxLogSize {
		if err := w.rotate(); err != nil {
			if w.file != nil {
				fmt.Fprintln(os.Stderr, "close file failed after 10 Mib:", err)
			}
			return nil, err
		}
	}

	return w, nil
}

func (w *rotatingFile) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if int64(len(p)) > maxLogSize {
		return 0, fmt.Errorf("log entry exceeds 10 MiB")
	}

	if w.file == nil {
		return 0, fmt.Errorf("log file is not open")
	}

	if w.size+int64(len(p)) > maxLogSize {
		if err := w.rotate(); err != nil {
			return 0, err
		}
	}

	n, err := w.file.Write(p)
	w.size += int64(n)
	return n, err
}

func (w *rotatingFile) rotate() error {
	backup := fmt.Sprintf(
		"%s.%s.bak",
		w.filename,
		time.Now().UTC().Format("20060102T150405.000000000Z"),
	)

	// Close first: required on Windows.
	if err := w.file.Close(); err != nil {
		return err
	}
	w.file = nil

	if err := os.Rename(w.filename, backup); err != nil {
		w.reopen()
		return err
	}

	f, err := os.OpenFile(
		w.filename,
		os.O_CREATE|os.O_WRONLY|os.O_TRUNC,
		0644,
	)
	if err != nil {
		_ = os.Rename(backup, w.filename)
		w.reopen()
		return err
	}

	w.file = f
	w.size = 0
	return nil
}

func (w *rotatingFile) reopen() {
	f, err := os.OpenFile(
		w.filename,
		os.O_CREATE|os.O_WRONLY|os.O_APPEND,
		0644,
	)
	if err != nil {
		return
	}

	w.file = f
	if info, err := f.Stat(); err == nil {
		w.size = info.Size()
	}
}

func newLogger(output io.Writer, level slog.Level) *slog.Logger {
	return slog.New(slog.NewJSONHandler(output, &slog.HandlerOptions{
		Level: level,
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			switch a.Key {
			case slog.TimeKey:
				a.Key = "timestamp"
			case slog.MessageKey:
				a.Key = "message"
			case slog.LevelKey:
				if lvl, ok := a.Value.Any().(slog.Level); ok {
					return slog.String("level", levelName(lvl))
				}
			}
			return a
		},
	}))
}

func parseLevel(value string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "trace":
		return levelTrace
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	case "fatal":
		return levelFatal
	case "panic":
		return levelPanic
	default:
		return slog.LevelInfo
	}
}

func levelName(level slog.Level) string {
	switch level {
	case levelTrace:
		return "trace"
	case slog.LevelDebug:
		return "debug"
	case slog.LevelInfo:
		return "info"
	case slog.LevelWarn:
		return "warn"
	case slog.LevelError:
		return "error"
	case levelFatal:
		return "fatal"
	case levelPanic:
		return "panic"
	default:
		return strings.ToLower(level.String())
	}
}

func emit(level slog.Level, message string) {
	logger := active.Load()
	ctx := context.Background()
	handler := logger.Handler()

	if !handler.Enabled(ctx, level) {
		return
	}

	var pcs [1]uintptr
	runtime.Callers(3, pcs[:])
	frame, _ := runtime.CallersFrames(pcs[:]).Next()

	record := slog.NewRecord(time.Now(), level, message, pcs[0])
	record.AddAttrs(
		slog.String("function", path.Base(frame.Function)),
		slog.String("file", filepath.Base(frame.File)),
		slog.Int("line", frame.Line),
	)

	if err := handler.Handle(ctx, record); err != nil {
		fmt.Fprintln(os.Stderr, "logger write failed:", err)
	}
}

//goland:noinspection GoUnusedExportedFunction
func Debug(message string) {
	emit(slog.LevelDebug, message)
}

//goland:noinspection GoUnusedExportedFunction
func Debugf(format string, args ...any) {
	if active.Load().Enabled(context.Background(), slog.LevelDebug) {
		emit(slog.LevelDebug, fmt.Sprintf(format, args...))
	}
}

//goland:noinspection GoUnusedExportedFunction
func Info(message string) {
	emit(slog.LevelInfo, message)
}

//goland:noinspection GoUnusedExportedFunction
func Infof(format string, args ...any) {
	if active.Load().Enabled(context.Background(), slog.LevelInfo) {
		emit(slog.LevelInfo, fmt.Sprintf(format, args...))
	}
}

//goland:noinspection GoUnusedExportedFunction
func Warn(message string) {
	emit(slog.LevelWarn, message)
}

//goland:noinspection GoUnusedExportedFunction
func Warnf(format string, args ...any) {
	if active.Load().Enabled(context.Background(), slog.LevelWarn) {
		emit(slog.LevelWarn, fmt.Sprintf(format, args...))
	}
}

//goland:noinspection GoUnusedExportedFunction
func Error(message string) {
	emit(slog.LevelError, message)
}

//goland:noinspection GoUnusedExportedFunction
func Errorf(format string, args ...any) {
	if active.Load().Enabled(context.Background(), slog.LevelError) {
		emit(slog.LevelError, fmt.Sprintf(format, args...))
	}
}

//goland:noinspection GoUnusedExportedFunction
func Traceln(args ...any) {
	if active.Load().Enabled(context.Background(), levelTrace) {
		emit(levelTrace, strings.TrimSuffix(fmt.Sprintln(args...), "\n"))
	}
}

//goland:noinspection GoUnusedExportedFunction
func Panic(args ...any) {
	message := fmt.Sprint(args...)
	emit(levelPanic, message)
	panic(message)
}

//goland:noinspection GoUnusedExportedFunction
func Panicf(format string, args ...any) {
	message := fmt.Sprintf(format, args...)
	emit(levelPanic, message)
	panic(message)
}

//goland:noinspection GoUnusedExportedFunction
func Fatalf(format string, args ...any) {
	emit(levelFatal, fmt.Sprintf(format, args...))
	os.Exit(1)
}
