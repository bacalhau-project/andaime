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
	GlobalLogPath             string = "/tmp/andaime.log"
	GlobalLogLevel            string
)

var (
	colorReset  = "\033[0m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
)

func CmdLog(msg string, level zapcore.Level) {
	timestamp := time.Now().Format("15:04:05")
	var colorCode string
	switch level {
	case zapcore.InfoLevel:
		colorCode = colorGreen
	case zapcore.WarnLevel:
		colorCode = colorYellow
	case zapcore.DebugLevel:
		colorCode = colorBlue
	default:
		colorCode = colorReset
	}
	fmt.Printf("%s[%s]%s %s\n", colorCode, timestamp, colorReset, msg)
}

// Logger is a wrapper around zap.Logger
type Logger struct {
	*zap.Logger
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
func InitProduction(enableConsole bool, enableFile bool) {
	once.Do(func() {
		var cores []zapcore.Core

		logPath := viper.GetString("general.log_path")
		if logPath != "" {
			GlobalLogPath = logPath
		}
		logLevelString := viper.GetString("general.log_level")
		if logLevelString != "" {
			GlobalLogLevel = logLevelString
		}
		logLevel := getLogLevel(GlobalLogLevel)

		if enableConsole {
			consoleCore := zapcore.NewCore(
				zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig()),
				zapcore.Lock(os.Stdout),
				logLevel,
			)
			cores = append(cores, consoleCore)
		}

		if enableFile {
			fileEncoderConfig := zap.NewProductionEncoderConfig()
			fileEncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

			debugFile, err := os.OpenFile(GlobalLogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
			if err == nil {
				fileCore := zapcore.NewCore(
					zapcore.NewJSONEncoder(fileEncoderConfig),
					zapcore.AddSync(debugFile),
					logLevel,
				)
				cores = append(cores, fileCore)
			}
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
		InitProduction(GlobalEnableConsoleLogger, GlobalEnableFileLogger)
	}
	l := &Logger{Logger: globalLogger}
	if l.Logger == nil {
		// If globalLogger is still nil after initialization, create a no-op logger
		l = NewNopLogger()
	}
	l.Debug("Logger accessed", zap.Bool("level", l.Core().Enabled(zapcore.DebugLevel)))
	return l
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
		InitProduction(GlobalEnableConsoleLogger, GlobalEnableFileLogger)
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
	InitProduction(false, true)
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

// LogAzureAPIEnd logs the end of an Azure API operation
func LogAzureAPIEnd(operation string, err error) {
	log := Get()
	if err != nil {
		log.Infof("Azure API operation failed: %s. Error: %v", operation, err)
	} else {
		log.Infof("Azure API operation completed successfully: %s", operation)
	}
}

func DebugPrint(msg string) {
	if globalLogger == nil {
		InitProduction(false, true)
	}
	globalLogger.Debug(msg)
}

func LogInitialization(msg string) {
	if globalLogger == nil {
		InitProduction(false, true)
	}
	globalLogger.Info(msg)
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
		fmt.Fprintf(os.Stderr, "Error writing to debug log file %s: %v\n", debugFilePath, err)
	}

	// Check if the file was actually written to
	fileInfo, err := os.Stat(debugFilePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error checking debug log file %s: %v\n", debugFilePath, err)
	} else if fileInfo.Size() == 0 {
		fmt.Fprintf(os.Stderr, "Warning: Debug log file %s is empty after write attempt\n", debugFilePath)
	}
}
