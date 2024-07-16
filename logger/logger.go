package logger

import (
	"fmt"
	"os"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest"
)

var (
	globalLogger *zap.Logger
	once         sync.Once
)

// Logger is a wrapper around zap.Logger
type Logger struct {
	*zap.Logger
}

// InitProduction initializes the global logger with production configuration
func InitProduction() {
	once.Do(func() {
		logger, err := zap.NewProduction()
		if err != nil {
			fmt.Printf("Failed to initialize production logger: %v\n", err)
			os.Exit(1)
		}
		globalLogger = logger
	})
}

// InitTest initializes the global logger for testing
func InitTest(tb zaptest.TestingT) {
	once.Do(func() {
		globalLogger = zaptest.NewLogger(tb)
	})
}

// Get returns the global logger instance
func Get() *Logger {
	if globalLogger == nil {
		InitProduction() // Default to production logger if not initialized
	}
	return &Logger{globalLogger}
}

// With creates a child logger and adds structured context to it
func (l *Logger) With(fields ...zap.Field) *Logger {
	return &Logger{l.Logger.With(fields...)}
}

// Debug logs a message at DebugLevel
func (l *Logger) Debug(msg string, fields ...zap.Field) {
	l.Logger.Debug(msg, fields...)
}

// Info logs a message at InfoLevel
func (l *Logger) Info(msg string, fields ...zap.Field) {
	l.Logger.Info(msg, fields...)
}

// Warn logs a message at WarnLevel
func (l *Logger) Warn(msg string, fields ...zap.Field) {
	l.Logger.Warn(msg, fields...)
}

// Error logs a message at ErrorLevel
func (l *Logger) Error(msg string, fields ...zap.Field) {
	l.Logger.Error(msg, fields...)
}

// Fatal logs a message at FatalLevel and then calls os.Exit(1)
func (l *Logger) Fatal(msg string, fields ...zap.Field) {
	l.Logger.Fatal(msg, fields...)
}

// Sync flushes any buffered log entries
func (l *Logger) Sync() error {
	return l.Logger.Sync()
}

// NewNopLogger returns a no-op Logger
func NewNopLogger() *Logger {
	return &Logger{zap.NewNop()}
}

// SetLevel sets the logging level for the global logger
func SetLevel(level zapcore.Level) {
	if globalLogger == nil {
		InitProduction()
	}
	globalLogger = globalLogger.WithOptions(zap.IncreaseLevel(level))
}

// Fields is a type alias for zap.Field for convenience
type Field = zap.Field

// Common field constructors
var (
	String  = zap.String
	Int     = zap.Int
	Float64 = zap.Float64
	Bool    = zap.Bool
	Error   = zap.Error
	Any     = zap.Any
)
