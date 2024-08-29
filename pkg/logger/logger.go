package logger

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest"

	"github.com/spf13/viper"
)

var (
	profileFilePath string
)

var (
	globalLogger *zap.Logger
	once         sync.Once
	outputFormat string        = "text"
	DEBUG        zapcore.Level = zapcore.DebugLevel
	INFO         zapcore.Level = zapcore.InfoLevel
	WARN         zapcore.Level = zapcore.WarnLevel
	ERROR        zapcore.Level = zapcore.ErrorLevel

	GlobalEnableConsoleLogger bool
	GlobalEnableFileLogger    bool
	GlobalEnableBufferLogger  bool
	GlobalLogPath             string = "/tmp/andaime.log"
	GlobalLogLevel            string = "info"
	GlobalInstantSync         bool
	GlobalLoggedBuffer        strings.Builder
	GlobalLoggedBufferSize    int = 8192
	GlobalLogFile             *os.File
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
	GlobalEnableBufferLogger = false
	GlobalLogPath = "/tmp/andaime.log"
	GlobalLogLevel = "info"
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
		fmt.Printf(
			"Initializing logger with: Console=%v, File=%v, Buffer=%v, LogPath=%s, LogLevel=%s\n",
			GlobalEnableConsoleLogger,
			GlobalEnableFileLogger,
			GlobalEnableBufferLogger,
			GlobalLogPath,
			GlobalLogLevel,
		)

		logPath := viper.GetString("general.log_path")
		if logPath != "" {
			GlobalLogPath = logPath
		}

		// Prioritize LOG_LEVEL environment variable
		envLogLevel := os.Getenv("LOG_LEVEL")
		if envLogLevel != "" {
			GlobalLogLevel = envLogLevel
		} else {
			logLevelString := viper.GetString("general.log_level")
			if logLevelString != "" {
				GlobalLogLevel = logLevelString
			}
		}

		if GlobalLogLevel == "" {
			GlobalLogLevel = "info"
		}
		logLevel := getZapLevel(GlobalLogLevel)
		atom := zap.NewAtomicLevelAt(logLevel)

		encoderConfig := zapcore.EncoderConfig{
			TimeKey:  "time",
			LevelKey: "level",
			// NameKey:    "logger",
			// CallerKey:  "caller",
			MessageKey: "message",
			// StacktraceKey: "stacktrace",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.LowercaseLevelEncoder,
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeDuration: zapcore.SecondsDurationEncoder,
			// EncodeCaller:   zapcore.ShortCallerEncoder,
		}

		var cores []zapcore.Core

		if GlobalEnableFileLogger {
			var err error
			GlobalLogFile, err = os.OpenFile(
				GlobalLogPath,
				os.O_CREATE|os.O_WRONLY|os.O_APPEND,
				0666,
			)
			if err == nil {
				fileWriter := zapcore.AddSync(GlobalLogFile)
				cores = append(cores, zapcore.NewCore(
					zapcore.NewConsoleEncoder(encoderConfig),
					fileWriter,
					atom,
				))
			} else {
				fmt.Printf("Error opening log file: %v\n", err)
			}
		}

		// if GlobalEnableBufferLogger {
		// 	cores = append(cores, zapcore.NewCore(
		// 		zapcore.NewConsoleEncoder(encoderConfig),
		// 		zapcore.AddSync(&GlobalLoggedBuffer),
		// 		atom,
		// 	))
		// }

		// if GlobalEnableConsoleLogger {
		// 	// If console logging is enabled, add a console core
		// 	consoleEncoder := zapcore.NewConsoleEncoder(encoderConfig)
		// 	cores = append(cores, zapcore.NewCore(consoleEncoder, zapcore.AddSync(os.Stdout), atom))
		// }

		core := zapcore.NewTee(cores...)
		// globalLogger = zap.New(core, zap.AddCaller())

		globalLogger = zap.New(core)
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

// Get returns the global logger instance
func Get() *Logger {
	if globalLogger == nil {
		InitProduction()
	}
	l := &Logger{Logger: globalLogger, verbose: false}
	if l.Logger == nil {
		l = NewNopLogger()
	}
	return l
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

func (l *Logger) Debug(msg string) {
	l.Logger.Debug(msg)
}

func (l *Logger) Info(msg string) {
	l.Logger.Info(msg)
}

func (l *Logger) Warn(msg string) {
	l.Logger.Warn(msg)
}

func (l *Logger) Error(msg string) {
	l.Logger.Error(msg)
}

func (l *Logger) Fatal(msg string) {
	l.Logger.Fatal(msg)
}

func (l *Logger) Debugf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	l.Logger.Debug(msg)
	l.syncIfNeeded()
}

func (l *Logger) Infof(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	l.Logger.Info(msg)
	l.syncIfNeeded()
}

func (l *Logger) Warnf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	l.Logger.Warn(msg)
	l.syncIfNeeded()
}

func (l *Logger) Errorf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	l.Logger.Error(msg)
	l.syncIfNeeded()
}

func (l *Logger) Fatalf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	l.Logger.Fatal(msg)
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
	l := Get()
	if GlobalLogFile == nil {
		l.Errorf("Error: GlobalLogFile is nil")
		writeToDebugLog("Error: GlobalLogFile is nil in GetLastLines")
		return make([]string, n) // Return an empty slice with length n
	}

	// Open the file for reading
	file, err := os.Open(GlobalLogPath)
	if err != nil {
		l.Errorf("Failed to open GlobalLogFile: %v", err)
		writeToDebugLog(fmt.Sprintf("Failed to open GlobalLogFile: %v", err))
		return make([]string, n)
	}
	defer file.Close()

	// Read the entire file content
	content, err := io.ReadAll(file)
	if err != nil {
		l.Errorf("Error reading GlobalLogFile: %v", err)
		writeToDebugLog(fmt.Sprintf("Error reading GlobalLogFile: %v", err))
		return make([]string, n)
	}

	// Split the content into lines
	lines := strings.Split(string(content), "\n")

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

func writeToDebugLog(message string) {
	debugFilePath := "/tmp/andaime-debug.log"
	debugFile, err := os.OpenFile(debugFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
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

func WriteToDebugLog(message string) {
	writeToDebugLog(message)
}

func WriteProfileInfo(info string) {
	if profileFilePath == "" {
		timestamp := time.Now().Format("20060102-150405")
		profileFilePath = fmt.Sprintf("/tmp/andaime-profile-%s.log", timestamp)
	}

	file, err := os.OpenFile(profileFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
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
