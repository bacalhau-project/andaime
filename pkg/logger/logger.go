package logger

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest"
)

var (
	globalLogger *zap.Logger
	once         sync.Once
	outputFormat string        = "text"
	DEBUG        zapcore.Level = zapcore.DebugLevel
	INFO         zapcore.Level = zapcore.InfoLevel
	WARN         zapcore.Level = zapcore.WarnLevel
	ERROR        zapcore.Level = zapcore.ErrorLevel
)

// Logger is a wrapper around zap.Logger
type Logger struct {
	*zap.Logger
}

// getLogLevel reads the LOG_LEVEL environment variable and returns the corresponding zapcore.Level.
func getLogLevel() zapcore.Level {
	logLevelEnv := os.Getenv("LOG_LEVEL")
	switch strings.ToUpper(logLevelEnv) {
	case "DEBUG":
		return zapcore.DebugLevel
	case "INFO":
		return zapcore.InfoLevel
	case "WARN":
		return zapcore.WarnLevel
	case "ERROR":
		return zapcore.ErrorLevel
	default:
		return zapcore.InfoLevel // Default to info level if LOG_LEVEL is not set or recognized
	}
}

// InitProduction initializes the global logger for production use.
func InitProduction() {
	once.Do(func() {
		logLevel := getLogLevel()

		var config zap.Config
		var core zapcore.Core

		switch logLevel {
		case zapcore.DebugLevel:
			config = zap.NewDevelopmentConfig()

			// Console encoder config (with color)
			consoleEncoderConfig := zap.NewDevelopmentEncoderConfig()
			consoleEncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
			consoleEncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

			// File encoder config (without color)
			fileEncoderConfig := zap.NewDevelopmentEncoderConfig()
			fileEncoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder

			// Set up file output for debug mode
			debugFile, err := os.OpenFile("debug.log", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
			if err != nil {
				fmt.Printf("Failed to open debug log file: %v\n", err)
				os.Exit(1)
			}

			// Create cores for console and file
			consoleCore := zapcore.NewCore(
				zapcore.NewConsoleEncoder(consoleEncoderConfig),
				zapcore.Lock(os.Stdout),
				config.Level,
			)
			fileCore := zapcore.NewCore(
				zapcore.NewConsoleEncoder(fileEncoderConfig),
				zapcore.AddSync(debugFile),
				config.Level,
			)

			// Combine cores
			core = zapcore.NewTee(consoleCore, fileCore)

		case zapcore.InfoLevel, zapcore.WarnLevel, zapcore.ErrorLevel:
			config = zap.NewProductionConfig()
			core = zapcore.NewCore(
				zapcore.NewJSONEncoder(config.EncoderConfig),
				zapcore.Lock(os.Stdout),
				config.Level,
			)
		default:
			config = zap.NewProductionConfig()
			core = zapcore.NewCore(
				zapcore.NewJSONEncoder(config.EncoderConfig),
				zapcore.Lock(os.Stdout),
				config.Level,
			)
		}

		logger := zap.New(core)

		// Verify log level
		if logger.Core().Enabled(logLevel) {
			logger.Info("Logger initialized", zap.String("level", logLevel.String()))
		} else {
			fmt.Printf("Warning: Logger not enabled for level %s\n", logLevel)
		}

		globalLogger = logger
	})
}

type testingWriter struct {
	tb zaptest.TestingT
}

func (tw *testingWriter) Write(p []byte) (n int, err error) {
	// Attempt to assert tb to *testing.T to access the Log method directly.
	if t, ok := tw.tb.(*testing.T); ok {
		t.Log(string(p))
	} else {
		fmt.Print(string(p))
	}
	return len(p), nil
}

// InitTest initializes the global logger for testing, respecting LOG_LEVEL.
func InitTest(tb zaptest.TestingT) {
	once.Do(func() {
		logLevel := getLogLevel()

		core := zapcore.NewCore(
			zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig()),
			zapcore.AddSync(&testingWriter{tb: tb}),
			zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
				return lvl >= logLevel
			}),
		)

		globalLogger = zap.New(core)
	})
}

// Get returns the global logger instance
func Get() *Logger {
	if globalLogger == nil {
		InitProduction() // Default to production logger if not initialized
	}
	logger := &Logger{globalLogger}
	logger.Debug("Logger initialized", zap.Bool("level", globalLogger.Core().Enabled(zapcore.DebugLevel)))
	return logger
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

// Debugf logs a formatted message at DebugLevel
func (l *Logger) Debugf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	l.Logger.Debug(msg)
}

// Infof logs a formatted message at InfoLevel
func (l *Logger) Infof(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	l.Logger.Info(msg)
}

// Warnf logs a formatted message at WarnLevel
func (l *Logger) Warnf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	l.Logger.Warn(msg)
}

// Errorf logs a formatted message at ErrorLevel
func (l *Logger) Errorf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	l.Logger.Error(msg)
}

// Fatalf logs a formatted message at FatalLevel and then calls os.Exit(1)
func (l *Logger) Fatalf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	l.Logger.Fatal(msg)
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

// SetOutputFormat sets the output format for the logger
func SetOutputFormat(format string) {
	if format != "text" && format != "json" {
		fmt.Printf("Invalid output format: %s. Using default format: text\n", format)
		outputFormat = "text"
	} else {
		outputFormat = format
	}
	InitProduction()
}

// LogAzureAPIStart logs the start of an Azure API operation
func LogAzureAPIStart(operation string) {
	if globalLogger != nil {
		globalLogger.Info("Starting Azure API operation", zap.String("operation", operation))
	}
}

// LogAzureAPIEnd logs the end of an Azure API operation
func LogAzureAPIEnd(operation string, err error) {
	if globalLogger != nil {
		if err != nil {
			globalLogger.Info("Azure API operation failed", zap.String("operation", operation), zap.Error(err))
		} else {
			globalLogger.Info("Azure API operation completed successfully", zap.String("operation", operation))
		}
	}
}

// Fields is a type alias for zap.Field for convenience
type Field = zap.Field

// Common field constructors
var (
	ZapString  = zap.String
	ZapInt     = zap.Int
	ZapFloat64 = zap.Float64
	ZapBool    = zap.Bool
	ZapError   = zap.Error
	ZapAny     = zap.Any
)
