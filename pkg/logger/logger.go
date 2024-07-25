package logger

import (
	"bufio"
	"fmt"
	"os"
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
	fmt.Fprintf(os.Stderr, "DEBUG: Entering InitProduction (enableConsole: %v, enableFile: %v)\n", enableConsole, enableFile)
	once.Do(func() {
		fmt.Fprintf(os.Stderr, "DEBUG: Inside once.Do\n")

		fmt.Fprintf(os.Stderr, "DEBUG: Viper settings:\n")
		fmt.Fprintf(os.Stderr, "  general.enable_console_logger: %v\n", viper.GetBool("general.enable_console_logger"))
		fmt.Fprintf(os.Stderr, "  general.enable_file_logger: %v\n", viper.GetBool("general.enable_file_logger"))
		fmt.Fprintf(os.Stderr, "  general.log_path: %s\n", viper.GetString("general.log_path"))
		fmt.Fprintf(os.Stderr, "  general.log_level: %s\n", viper.GetString("general.log_level"))

		if viper.GetBool("general.enable_console_logger") {
			GlobalEnableConsoleLogger = viper.GetBool("general.enable_console_logger")
			fmt.Fprintf(os.Stderr, "DEBUG: GlobalEnableConsoleLogger set to %v\n", GlobalEnableConsoleLogger)
		}
		if viper.GetBool("general.enable_file_logger") {
			GlobalEnableFileLogger = viper.GetBool("general.enable_file_logger")
			fmt.Fprintf(os.Stderr, "DEBUG: GlobalEnableFileLogger set to %v\n", GlobalEnableFileLogger)
		}
		if viper.GetString("general.log_path") != "" {
			GlobalLogPath = viper.GetString("general.log_path")
			fmt.Fprintf(os.Stderr, "DEBUG: GlobalLogPath set to %s\n", GlobalLogPath)
		}
		if viper.GetString("general.log_level") != "" {
			GlobalLogLevel = viper.GetString("general.log_level")
			fmt.Fprintf(os.Stderr, "DEBUG: GlobalLogLevel set to %s\n", GlobalLogLevel)
		}

		logLevel := getLogLevel(GlobalLogLevel)
		fmt.Fprintf(os.Stderr, "DEBUG: Log level set to %v\n", logLevel)

		var cores []zapcore.Core

		if enableConsole {
			fmt.Fprintf(os.Stderr, "DEBUG: Enabling console logging\n")
			// Custom encoder for console output
			customConsoleEncoder := zapcore.NewConsoleEncoder(zapcore.EncoderConfig{
				TimeKey:        "",
				LevelKey:       "",
				NameKey:        "",
				CallerKey:      "",
				FunctionKey:    zapcore.OmitKey,
				MessageKey:     "message",
				StacktraceKey:  "",
				LineEnding:     zapcore.DefaultLineEnding,
				EncodeLevel:    zapcore.CapitalColorLevelEncoder,
				EncodeTime:     zapcore.ISO8601TimeEncoder,
				EncodeDuration: zapcore.StringDurationEncoder,
				EncodeCaller:   zapcore.ShortCallerEncoder,
			})

			// Create console core (always used)
			consoleCore := zapcore.NewCore(
				customConsoleEncoder,
				zapcore.Lock(os.Stdout),
				logLevel,
			)
			cores = append(cores, consoleCore)
		}

		if enableFile {
			fmt.Fprintf(os.Stderr, "DEBUG: Enabling file logging\n")
			// File encoder config
			fileEncoderConfig := zap.NewProductionEncoderConfig()
			fileEncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

			// Set up file output for debug mode
			fmt.Fprintf(os.Stderr, "DEBUG: Attempting to open log file: %s\n", GlobalLogPath)
			debugFile, err := os.OpenFile(GlobalLogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
			if err != nil {
				fmt.Fprintf(os.Stderr, "DEBUG: Failed to open debug log file: %v\n", err)
			} else {
				fmt.Fprintf(os.Stderr, "DEBUG: Successfully opened log file\n")
				fileCore := zapcore.NewCore(
					zapcore.NewJSONEncoder(fileEncoderConfig),
					zapcore.AddSync(debugFile),
					logLevel, // Use the configured log level instead of always using DebugLevel
				)
				cores = append(cores, fileCore)
				fmt.Fprintf(os.Stderr, "DEBUG: Added file core to cores slice\n")
			}
		}
		// Combine cores
		fmt.Fprintf(os.Stderr, "DEBUG: Combining cores\n")
		core := zapcore.NewTee(cores...)

		fmt.Fprintf(os.Stderr, "DEBUG: Creating new logger\n")
		logger := zap.New(core)

		globalLogger = logger
		fmt.Fprintf(os.Stderr, "DEBUG: Global logger set\n")
	})
	fmt.Fprintf(os.Stderr, "DEBUG: Exiting InitProduction\n")
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
	fmt.Fprintf(os.Stderr, "DEBUG: Entering Get()\n")
	if globalLogger == nil {
		fmt.Fprintf(os.Stderr, "DEBUG: globalLogger is nil, initializing production logger\n")
		InitProduction(GlobalEnableConsoleLogger, GlobalEnableFileLogger)
	}
	logger := &Logger{Logger: globalLogger}
	fmt.Fprintf(os.Stderr, "DEBUG: Logger created, debug level enabled: %v\n", globalLogger.Core().Enabled(zapcore.DebugLevel))
	logger.Debug("Logger initialized", zap.Bool("level", globalLogger.Core().Enabled(zapcore.DebugLevel)))
	fmt.Fprintf(os.Stderr, "DEBUG: Exiting Get()\n")
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
		InitProduction(false, true)
	}
	globalLogger = globalLogger.WithOptions(zap.IncreaseLevel(level))
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
	l.Debugf("GetLastLines called with filepath: '%s' and n: %d", filepath, n)
	
	if filepath == "" {
		l.Errorf("Error: filepath is empty")
		return nil
	}

	l.Debugf("Attempting to open file: '%s'", filepath)
	file, err := os.Open(filepath)
	if err != nil {
		l.Errorf("Error opening file '%s': %v", filepath, err)
		return nil
	}
	defer file.Close()
	l.Debugf("File opened successfully: '%s'", filepath)

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
	l.Debugf("Read %d lines from file '%s'", lineCount, filepath)

	if err := scanner.Err(); err != nil {
		l.Errorf("Error reading file '%s': %v", filepath, err)
	}

	l.Debugf("Returning %d lines from file '%s'", len(lines), filepath)
	return lines
}
