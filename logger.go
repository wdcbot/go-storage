package storage

import (
	"context"
	"io"
	"log"
	"os"
	"time"
)

// Logger interface for custom logging.
type Logger interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
}

// Default logger (can be replaced).
var defaultLogger Logger = &nopLogger{}

// SetLogger sets the global logger.
func SetLogger(l Logger) {
	if l == nil {
		defaultLogger = &nopLogger{}
		return
	}
	defaultLogger = l
}

// EnableDebugLog enables simple debug logging to stderr.
func EnableDebugLog() {
	defaultLogger = &stdLogger{level: "debug"}
}

// nopLogger is a no-op logger.
type nopLogger struct{}

func (l *nopLogger) Debug(msg string, args ...any) {}
func (l *nopLogger) Info(msg string, args ...any)  {}
func (l *nopLogger) Warn(msg string, args ...any)  {}
func (l *nopLogger) Error(msg string, args ...any) {}

// stdLogger is a simple logger using standard log package.
type stdLogger struct {
	level string
}

func (l *stdLogger) Debug(msg string, args ...any) {
	if l.level == "debug" {
		log.Printf("[DEBUG] storage: "+msg, args...)
	}
}

func (l *stdLogger) Info(msg string, args ...any) {
	log.Printf("[INFO] storage: "+msg, args...)
}

func (l *stdLogger) Warn(msg string, args ...any) {
	log.Printf("[WARN] storage: "+msg, args...)
}

func (l *stdLogger) Error(msg string, args ...any) {
	log.Printf("[ERROR] storage: "+msg, args...)
}

// SlogAdapter adapts slog.Logger to our Logger interface.
// Usage: storage.SetLogger(storage.NewSlogAdapter(slog.Default()))
type SlogAdapter struct {
	logger interface {
		Debug(msg string, args ...any)
		Info(msg string, args ...any)
		Warn(msg string, args ...any)
		Error(msg string, args ...any)
	}
}

func NewSlogAdapter(l interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
}) *SlogAdapter {
	return &SlogAdapter{logger: l}
}

func (a *SlogAdapter) Debug(msg string, args ...any) { a.logger.Debug(msg, args...) }
func (a *SlogAdapter) Info(msg string, args ...any)  { a.logger.Info(msg, args...) }
func (a *SlogAdapter) Warn(msg string, args ...any)  { a.logger.Warn(msg, args...) }
func (a *SlogAdapter) Error(msg string, args ...any) { a.logger.Error(msg, args...) }

// LoggingStorage wraps a Storage with logging.
type LoggingStorage struct {
	Storage
	name   string
	logger Logger
}

// WrapWithLogging wraps a storage with logging.
func WrapWithLogging(s Storage, name string, logger Logger) *LoggingStorage {
	if logger == nil {
		logger = defaultLogger
	}
	return &LoggingStorage{Storage: s, name: name, logger: logger}
}

func (l *LoggingStorage) Upload(ctx context.Context, key string, reader io.Reader, opts ...UploadOption) (*UploadResult, error) {
	start := time.Now()
	result, err := l.Storage.Upload(ctx, key, reader, opts...)
	duration := time.Since(start)

	if err != nil {
		l.logger.Error("upload failed: disk=%s key=%s duration=%s error=%v", l.name, key, duration, err)
	} else {
		l.logger.Debug("upload success: disk=%s key=%s size=%d duration=%s", l.name, key, result.Size, duration)
	}
	return result, err
}

func (l *LoggingStorage) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	start := time.Now()
	reader, err := l.Storage.Download(ctx, key)
	duration := time.Since(start)

	if err != nil {
		l.logger.Error("download failed: disk=%s key=%s duration=%s error=%v", l.name, key, duration, err)
	} else {
		l.logger.Debug("download success: disk=%s key=%s duration=%s", l.name, key, duration)
	}
	return reader, err
}

func (l *LoggingStorage) Delete(ctx context.Context, key string) error {
	start := time.Now()
	err := l.Storage.Delete(ctx, key)
	duration := time.Since(start)

	if err != nil {
		l.logger.Error("delete failed: disk=%s key=%s duration=%s error=%v", l.name, key, duration, err)
	} else {
		l.logger.Debug("delete success: disk=%s key=%s duration=%s", l.name, key, duration)
	}
	return err
}

// Debug returns true if STORAGE_DEBUG env is set.
func Debug() bool {
	return os.Getenv("STORAGE_DEBUG") != ""
}

func init() {
	if Debug() {
		EnableDebugLog()
	}
}
