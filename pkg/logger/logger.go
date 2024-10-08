package logger

import (
	"fmt"
	"io/fs"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest"

	"runtime/debug"

	"github.com/spf13/viper"
)

const (
	LogFilePermissions     = 0600
	DebugFilePermissions   = 0600
	InfoLogLevel           = "info"
	ProfileFilePermissions = 0600
)

var (
	profileFilePath string
)

var (
	globalLogger *zap.Logger
	loggerMutex  sync.RWMutex
	once         sync.Once
	DEBUG        zapcore.Level = zapcore.DebugLevel
	INFO         zapcore.Level = zapcore.InfoLevel
	WARN         zapcore.Level = zapcore.WarnLevel
	ERROR        zapcore.Level = zapcore.ErrorLevel

	GlobalEnableConsoleLogger bool
	GlobalEnableFileLogger    bool
	GlobalEnableBufferLogger  bool
	GlobalLogPath             string = "/tmp/andaime.log"
	GlobalLogLevel            string = InfoLogLevel
	GlobalInstantSync         bool
	GlobalLoggedBuffer        strings.Builder
	GlobalLoggedBufferSize    int = 8192
	GlobalLogFile             *os.File

	debugFilePermissions   = fs.FileMode(DebugFilePermissions)
	profileFilePermissions = fs.FileMode(ProfileFilePermissions)

	outputFormat = "text" //nolint:unused
)

// Logger is a wrapper around zap.Logger
type Logger struct {
	*zap.Logger
	verbose bool
}

func (l *Logger) SetVerbose(verbose bool) {
	l.verbose = verbose
}

func InitLoggerOutputs() {
	GlobalEnableConsoleLogger = false
	GlobalEnableFileLogger = true
	GlobalEnableBufferLogger = true
	GlobalLogPath = "/tmp/andaime.log"
	GlobalLogLevel = InfoLogLevel
	GlobalInstantSync = false

	if viper.IsSet("general.log_path") {
		GlobalLogPath = viper.GetString("general.log_path")
	}
	if viper.IsSet("general.log_level") {
		GlobalLogLevel = viper.GetString("general.log_level")
	}
	if viper.IsSet("general.enable_console_logger") {
		GlobalEnableConsoleLogger = viper.GetBool("general.enable_console_logger")
	}
	if viper.IsSet("general.enable_file_logger") {
		GlobalEnableFileLogger = viper.GetBool("general.enable_file_logger")
	}
	if viper.IsSet("general.enable_buffer_logger") {
		GlobalEnableBufferLogger = viper.GetBool("general.enable_buffer_logger")
	}
}

// getLogLevel reads the LOG_LEVEL environment variable and returns the corresponding zapcore.Level.
func getLogLevel(logLevel string) zapcore.Level {
	switch strings.ToUpper(logLevel) {
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
func InitProduction() {
	once.Do(func() {
		// Enable console logging by default
		GlobalEnableConsoleLogger = false
		GlobalEnableFileLogger = true
		GlobalEnableBufferLogger = true

		fmt.Printf(
			"Initializing logger with: Console=%v, File=%v, Buffer=%v, LogPath=%s, LogLevel=%s\n",
			GlobalEnableConsoleLogger,
			GlobalEnableFileLogger,
			GlobalEnableBufferLogger,
			GlobalLogPath,
			GlobalLogLevel,
		)

		if GlobalLogLevel == "" {
			GlobalLogLevel = InfoLogLevel
		}
		logLevel := getZapLevel(GlobalLogLevel)

		config := zap.NewProductionConfig()
		config.Level = zap.NewAtomicLevelAt(logLevel)

		var cores []zapcore.Core

		if GlobalEnableConsoleLogger {
			// Console logging with a more readable format
			consoleEncoderConfig := zap.NewDevelopmentEncoderConfig()
			consoleEncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
			consoleEncoderConfig.EncodeTime = zapcore.TimeEncoderOfLayout("2006-01-02-15:04:05")
			consoleEncoder := zapcore.NewConsoleEncoder(consoleEncoderConfig)
			cores = append(cores, zapcore.NewCore(
				consoleEncoder,
				zapcore.AddSync(os.Stdout),
				config.Level,
			))
		}

		// File logging with JSON format (if enabled)
		if GlobalEnableFileLogger {
			fileEncoderConfig := zapcore.EncoderConfig{
				TimeKey:        "time",
				LevelKey:       "level",
				NameKey:        "logger",
				CallerKey:      "caller",
				MessageKey:     "msg",
				StacktraceKey:  "stacktrace",
				LineEnding:     zapcore.DefaultLineEnding,
				EncodeLevel:    zapcore.LowercaseLevelEncoder,
				EncodeTime:     customTimeEncoder,
				EncodeDuration: zapcore.SecondsDurationEncoder,
				EncodeCaller:   zapcore.ShortCallerEncoder,
			}
			fileEncoder := zapcore.NewJSONEncoder(fileEncoderConfig)
			logFile, err := os.OpenFile(GlobalLogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				fmt.Printf("Failed to open log file: %v\n", err)
			} else {
				fmt.Printf("Successfully opened log file: %s\n", GlobalLogPath)
				cores = append(cores, zapcore.NewCore(
					fileEncoder,
					zapcore.AddSync(logFile),
					config.Level,
				))
			}
		}

		// Buffer logging with JSON format (if enabled)
		if GlobalEnableBufferLogger {
			bufferEncoderConfig := zapcore.EncoderConfig{
				TimeKey:        "time",
				LevelKey:       "level",
				MessageKey:     "msg",
				StacktraceKey:  "stacktrace",
				EncodeLevel:    zapcore.CapitalColorLevelEncoder,
				LineEnding:     zapcore.DefaultLineEnding,
				EncodeTime:     customTimeEncoder,
				EncodeDuration: zapcore.SecondsDurationEncoder,
				EncodeCaller:   zapcore.ShortCallerEncoder,
			}
			bufferEncoder := zapcore.NewConsoleEncoder(bufferEncoderConfig)
			cores = append(cores, zapcore.NewCore(
				bufferEncoder,
				zapcore.AddSync(&GlobalLoggedBuffer),
				config.Level,
			))
		}

		core := zapcore.NewTee(cores...)
		logger := zap.New(core, zap.AddCaller())

		globalLogger = logger.Named("andaime")

		fmt.Println("Logger initialized successfully")
	})
}

// NewTestLogger creates a new Logger instance for testing
func NewTestLogger(tb zaptest.TestingT) *Logger {
	core := zapcore.NewCore(
		zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig()),
		zapcore.AddSync(&testingWriter{tb: tb}),
		zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
			return lvl >= zapcore.DebugLevel
		}),
	)
	return &Logger{
		Logger:  zap.New(core),
		verbose: true,
	}
}

type testingWriter struct {
	tb zaptest.TestingT
}

func (tw *testingWriter) Write(p []byte) (n int, err error) {
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
		logLevel := getLogLevel("DEBUG")

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

// SetGlobalLogger sets the global logger instance
func SetGlobalLogger(logger *Logger) {
	loggerMutex.Lock()
	defer loggerMutex.Unlock()
	globalLogger = logger.Logger
}

// Get returns the global logger instance
func Get() *Logger {
	loggerMutex.Lock()
	defer loggerMutex.Unlock()

	if globalLogger == nil {
		InitProduction()
	}
	return &Logger{Logger: globalLogger, verbose: false}
}

func (l *Logger) syncIfNeeded() {
	if GlobalInstantSync {
		_ = l.Sync() // Ignore the error for now, but you might want to handle it in a production environment
	}
}

// With creates a child logger and adds structured context to it
func (l *Logger) With(fields ...zap.Field) *Logger {
	return &Logger{l.Logger.With(fields...), l.verbose}
}

func (l *Logger) Debug(msg string, fields ...zap.Field) {
	l.Logger.Debug(formatMessage(msg), fields...)
	l.syncIfNeeded()
}

func (l *Logger) Info(msg string, fields ...zap.Field) {
	l.Logger.Info(formatMessage(msg), fields...)
	l.syncIfNeeded()
}

func (l *Logger) Warn(msg string, fields ...zap.Field) {
	l.Logger.Warn(formatMessage(msg), fields...)
	l.syncIfNeeded()
}

func (l *Logger) Error(msg string, fields ...zap.Field) {
	l.Logger.Error(formatMessage(msg), fields...)
	l.syncIfNeeded()
}

func (l *Logger) Fatal(msg string, fields ...zap.Field) {
	l.Logger.Fatal(formatMessage(msg), fields...)
	// No need to sync here as Fatal will exit the program
}

func formatMessage(msg string) string {
	return strings.TrimPrefix(msg, "andaime\t")
}

func (l *Logger) Debugf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	l.Logger.Debug(formatMessage(msg))
	l.syncIfNeeded()
}

func (l *Logger) Infof(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	l.Logger.Info(formatMessage(msg))
	l.syncIfNeeded()
}

func (l *Logger) Warnf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	l.Logger.Warn(formatMessage(msg))
	l.syncIfNeeded()
}

func (l *Logger) Errorf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	l.Logger.Error(formatMessage(msg))
	l.syncIfNeeded()
}

func (l *Logger) Fatalf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	l.Logger.Fatal(formatMessage(msg))
	// No need to sync here as Fatalf will exit the program
}

// Sync flushes any buffered log entries
func (l *Logger) Sync() error {
	return l.Logger.Sync()
}

// NewNopLogger returns a no-op Logger
func NewNopLogger() *Logger {
	return &Logger{Logger: zap.NewNop(), verbose: false}
}

// SetLevel sets the logging level for the global logger
func SetLevel(level zapcore.Level) {
	if globalLogger == nil {
		InitProduction()
	}
	if globalLogger != nil {
		// Create a new logger with the desired level instead of modifying the existing one
		config := zap.NewProductionConfig()
		config.Level = zap.NewAtomicLevelAt(level)
		newLogger, err := config.Build()
		if err != nil {
			Get().Errorf("Failed to create new logger with level %v: %v", level, err)
			return
		}
		globalLogger = newLogger
	}
}

// SetOutputFormat sets the output format for the logger
func SetOutputFormat(format string) {
	if format != "text" && format != "json" {
		Get().Warnf("Invalid output format: %s. Using default format: text", format)
		outputFormat = "text"
	} else {
		outputFormat = format
	}
	InitProduction()
}

// LevelEnablerFunc is a wrapper to implement zapcore.LevelEnabler
type LevelEnablerFunc func(zapcore.Level) bool

// Enabled implements zapcore.LevelEnabler interface
func (f LevelEnablerFunc) Enabled(level zapcore.Level) bool {
	return f(level)
}

// LogAzureAPIStart logs the start of an Azure API operation
func LogAzureAPIStart(operation string) {
	log := Get()
	if globalLogger != nil {
		log.Infof("Starting Azure API operation: %s", operation)
	}
}

func DebugPrint(msg string) {
	if globalLogger == nil {
		InitProduction()
	}
	globalLogger.Debug(msg)
	if GlobalInstantSync {
		_ = globalLogger.Sync()
	}
}

func LogInitialization(msg string) {
	if globalLogger == nil {
		InitProduction()
	}
	globalLogger.Info(msg)
	if GlobalInstantSync {
		_ = globalLogger.Sync()
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

func GetLastLines(n int) []string {
	if GlobalEnableBufferLogger {
		return getLastLinesFromBuffer(n)
	}

	l := Get()
	l.Warnf("In-memory buffer logging is not enabled. Unable to retrieve last lines.")
	return []string{}
}

func writeToDebugLog(message string) {
	debugFilePath := "/tmp/andaime-debug.log"
	debugFile, err := os.OpenFile(
		debugFilePath,
		os.O_APPEND|os.O_CREATE|os.O_WRONLY,
		debugFilePermissions,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening debug log file %s: %v\n", debugFilePath, err)
		return
	}
	defer debugFile.Close()

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	_, err = fmt.Fprintf(debugFile, "[%s] %s\n", timestamp, message)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error writing to debug log file: %v\n", err)
	}
}

func getLastLinesFromBuffer(n int) []string {
	buffer := GlobalLoggedBuffer.String()
	lines := strings.Split(buffer, "\n")

	// Get the last n lines
	start := len(lines) - n
	if start < 0 {
		start = 0
	}
	lastLines := lines[start:]

	// Remove empty lines
	var result []string
	for _, line := range lastLines {
		if line != "" {
			result = append(result, line)
		}
	}

	return result
}

func WriteToDebugLog(message string) {
	writeToDebugLog(message)
}

func WriteProfileInfo(info string) {
	if profileFilePath == "" {
		timestamp := time.Now().Format("20060102-150405")
		profileFilePath = fmt.Sprintf("/tmp/andaime-profile-%s.log", timestamp)
	}

	file, err := os.OpenFile(
		profileFilePath,
		os.O_APPEND|os.O_CREATE|os.O_WRONLY,
		profileFilePermissions,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening profile log file %s: %v\n", profileFilePath, err)
		return
	}
	defer file.Close()

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	_, err = fmt.Fprintf(file, "[%s] %s\n", timestamp, info)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error writing to profile log file: %v\n", err)
	}
}

func GetProfileFilePath() string {
	return profileFilePath
}

// SetLogLevel changes the global log level
func SetLogLevel(level string) {
	GlobalLogLevel = level
	if globalLogger != nil {
		zapLevel := getZapLevel(level)
		globalLogger.Core().Enabled(zapLevel)
	}
}
func getZapLevel(level string) zapcore.Level {
	switch strings.ToLower(level) {
	case "debug":
		return zapcore.DebugLevel
	case "info":
		return zapcore.InfoLevel
	case "warn":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	default:
		fmt.Printf("Unrecognized log level '%s', defaulting to INFO\n", level)
		return zapcore.InfoLevel
	}
}

func customTimeEncoder(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(fmt.Sprintf("[%s]", t.Format("2006-01-02 15:04:05")))
}

// LogPanic logs the panic message and stack trace
func LogPanic(rec interface{}) {
	stack := debug.Stack()
	Get().Error("PANIC",
		zap.Any("recover", rec),
		zap.String("stack", string(stack)),
	)
}

// RecoverAndLog wraps a function with panic recovery and logging
func RecoverAndLog(f func()) {
	defer func() {
		if r := recover(); r != nil {
			LogPanic(r)
			panic(r) // re-panic after logging
		}
	}()
	f()
}
