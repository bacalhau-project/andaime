package logger

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest"

	"github.com/rivo/tview"
	"github.com/spf13/viper"
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
	GlobalLogLevel            string
	GlobalInstantSync         bool
	GlobalLoggedBuffer        strings.Builder
	GlobalLoggedBufferSize    int = 8192
)

// Logger is a wrapper around zap.Logger
type Logger struct {
	*zap.Logger
	verbose bool
}

func (l *Logger) SetVerbose(verbose bool) {
	l.verbose = verbose
}

type LogBoxWriter struct {
	LogBox *tview.TextView
	App    *tview.Application
}

func (w *LogBoxWriter) Write(p []byte) (n int, err error) {
	w.App.QueueUpdateDraw(func() {
		_, err = w.LogBox.Write(p)
	})
	return len(p), err
}

func InitLoggerOutputs() {
	GlobalEnableConsoleLogger = false
	GlobalEnableFileLogger = true
	GlobalEnableBufferLogger = true
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
		var cores []zapcore.Core

		logPath := viper.GetString("general.log_path")
		if logPath != "" {
			GlobalLogPath = logPath
		}
		if os.Getenv("LOG_LEVEL") != "" {
			GlobalLogLevel = os.Getenv("LOG_LEVEL")
		} else {
			logLevelString := viper.GetString("general.log_level")
			if logLevelString != "" {
				GlobalLogLevel = logLevelString
			}
		}

		logLevel := getLogLevel(GlobalLogLevel)

		if GlobalEnableConsoleLogger {
			// Add console logging
			consoleCore := zapcore.NewCore(
				zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig()),
				zapcore.AddSync(os.Stdout),
				logLevel,
			)
			cores = append(cores, consoleCore)
		}

		if GlobalEnableFileLogger {
			fileEncoderConfig := zap.NewProductionEncoderConfig()
			fileEncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

			debugFile, err := os.OpenFile(GlobalLogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
			if err == nil {
				fileCore := zapcore.NewCore(
					zapcore.NewConsoleEncoder(fileEncoderConfig),
					zapcore.AddSync(debugFile),
					logLevel,
				)
				cores = append(cores, fileCore)
			}
		}

		if GlobalEnableBufferLogger {
			bufferCore := zapcore.NewCore(
				zapcore.NewConsoleEncoder(zapcore.EncoderConfig{
					TimeKey:        "time",
					LevelKey:       "level",
					NameKey:        "logger",
					CallerKey:      "caller",
					MessageKey:     "message",
					StacktraceKey:  "stacktrace",
					LineEnding:     zapcore.DefaultLineEnding,
					EncodeLevel:    zapcore.LowercaseLevelEncoder,
					EncodeTime:     zapcore.ISO8601TimeEncoder,
					EncodeDuration: zapcore.SecondsDurationEncoder,
					EncodeCaller:   zapcore.ShortCallerEncoder,
				}),
				zapcore.AddSync(&GlobalLoggedBuffer),
				logLevel,
			)
			cores = append(cores, bufferCore)
		}

		core := zapcore.NewTee(cores...)
		globalLogger = zap.New(core)
	})

	if globalLogger == nil {
		// If globalLogger is still nil after initialization, create a no-op logger
		globalLogger = zap.NewNop()
	}
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
		// If globalLogger is still nil after initialization, create a no-op logger
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
	l.syncIfNeeded()
	if l.verbose {
		fmt.Println("DEBUG:", msg)
	}
}

func (l *Logger) Info(msg string) {
	l.Logger.Info(msg)
	l.syncIfNeeded()
	if l.verbose {
		fmt.Println("INFO:", msg)
	}
}

func (l *Logger) Warn(msg string) {
	l.Logger.Warn(msg)
	l.syncIfNeeded()
	if l.verbose {
		fmt.Println("WARN:", msg)
	}
}

func (l *Logger) Error(msg string) {
	l.Logger.Error(msg)
	l.syncIfNeeded()
	if l.verbose {
		fmt.Println("ERROR:", msg)
	}
}

func (l *Logger) Fatal(msg string) {
	l.Logger.Fatal(msg)
	// No need to sync here as Fatal will exit the program
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
		globalLogger = globalLogger.WithOptions(zap.IncreaseLevel(level))
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

func GetLastLines(filepath string, n int) []string {
	l := Get()
	if filepath == "" {
		l.Errorf("Error: filepath is empty")
		writeToDebugLog("Error: filepath is empty in GetLastLines")
		buf := make([]byte, 1024)
		runtime.Stack(buf, false)
		writeToDebugLog(fmt.Sprintf("Stack trace:\n%s", string(buf)))
		return []string{"Error: filepath is empty"}
	}

	file, err := os.Open(filepath)
	if err != nil {
		errMsg := fmt.Sprintf("Error opening file '%s': %v", filepath, err)
		l.Errorf(errMsg)
		writeToDebugLog(errMsg)
		return []string{errMsg}
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	lineCount := 0
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
		if len(lines) > n {
			lines = lines[1:]
		}
		lineCount++
	}

	if err := scanner.Err(); err != nil {
		errMsg := fmt.Sprintf("Error reading file '%s': %v", filepath, err)
		l.Errorf(errMsg)
		writeToDebugLog(errMsg)
		return append([]string{errMsg}, lines...)
	}

	return lines
}

func writeToDebugLog(message string) {
	debugFilePath := "/tmp/andaime.log"
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
func getZapLevel(level string) zapcore.Level {
	switch level {
	case "debug":
		return zapcore.DebugLevel
	case "info":
		return zapcore.InfoLevel
	case "warn":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	default:
		return zapcore.InfoLevel
	}
}
