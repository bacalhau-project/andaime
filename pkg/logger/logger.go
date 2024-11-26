package logger

import (
	"fmt"
	"io/fs"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"runtime/debug"

	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest"
)

// Constants
const (
	LogFilePermissions     = 0600
	DebugFilePermissions   = 0600
	ProfileFilePermissions = 0600
	InfoLogLevel           = "info"
)

// Global variables
var (
	globalLogger    *zap.Logger
	loggerMutex     sync.RWMutex
	once            sync.Once
	profileFilePath string

	// Log levels
	DEBUG zapcore.Level = zapcore.DebugLevel
	INFO  zapcore.Level = zapcore.InfoLevel
	WARN  zapcore.Level = zapcore.WarnLevel
	ERROR zapcore.Level = zapcore.ErrorLevel

	// Global settings
	GlobalEnableConsoleLogger bool
	GlobalEnableFileLogger    bool
	GlobalEnableBufferLogger  bool
	GlobalLogPath             string = "/tmp/andaime.log"
	GlobalLogLevel            string = InfoLogLevel
	GlobalInstantSync         bool
	GlobalLoggedBuffer        strings.Builder
	GlobalLoggedBufferSize    int = 8192
	GlobalLogFile             *os.File

	// File permissions
	debugFilePermissions   = fs.FileMode(DebugFilePermissions)
	profileFilePermissions = fs.FileMode(ProfileFilePermissions)
)

// Logger types
type Logger struct {
	*zap.Logger
	verbose bool
}

type TestLogger struct {
	*Logger
	t       *testing.T
	logs    *[]string // Change to pointer
	logLock *sync.Mutex
	buffer  *LogBuffer
}

type levelEnabler struct{}

func (l levelEnabler) Enabled(level zapcore.Level) bool {
	return level >= zapcore.DebugLevel
}

// Writer for test output
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

// Initialization functions
func InitLoggerOutputs() {
	GlobalEnableConsoleLogger = false
	GlobalEnableFileLogger = true
	GlobalEnableBufferLogger = true
	GlobalLogPath = "/tmp/andaime.log"
	GlobalLogLevel = InfoLogLevel
	GlobalInstantSync = false

	// Load settings from viper if available
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

func InitProduction() {
	once.Do(func() {
		if GlobalLogLevel == "" {
			GlobalLogLevel = InfoLogLevel
		}
		logLevel := getZapLevel(GlobalLogLevel)

		config := zap.NewProductionConfig()
		config.Level = zap.NewAtomicLevelAt(logLevel)
		var cores []zapcore.Core

		// Add console core if enabled
		if GlobalEnableConsoleLogger {
			cores = append(cores, createConsoleCore(config.Level))
		}

		// Add file core if enabled
		if GlobalEnableFileLogger {
			if fileCore, err := createFileCore(config.Level); err == nil {
				cores = append(cores, fileCore)
			}
		}

		// Add buffer core if enabled
		if GlobalEnableBufferLogger {
			cores = append(cores, createBufferCore(config.Level))
		}

		core := zapcore.NewTee(cores...)
		globalLogger = zap.New(core, zap.AddCaller()).Named("andaime")
	})
}

// Core creation helpers
func createConsoleCore(level zap.AtomicLevel) zapcore.Core {
	encoderConfig := zap.NewDevelopmentEncoderConfig()
	encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	encoderConfig.EncodeTime = zapcore.TimeEncoderOfLayout("2006-01-02-15:04:05")
	encoderConfig.LineEnding = "\n"
	return zapcore.NewCore(
		zapcore.NewConsoleEncoder(encoderConfig),
		zapcore.AddSync(os.Stdout),
		level,
	)
}

func createFileCore(level zap.AtomicLevel) (zapcore.Core, error) {
	encoderConfig := zapcore.EncoderConfig{
		// ... encoder config ...
	}

	logFile, err := os.OpenFile(GlobalLogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return nil, err
	}
	GlobalLogFile = logFile // Store for cleanup

	return zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		zapcore.AddSync(logFile),
		level,
	), nil
}

func createBufferCore(level zap.AtomicLevel) zapcore.Core {
	encoderConfig := zapcore.EncoderConfig{
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

	return zapcore.NewCore(
		zapcore.NewConsoleEncoder(encoderConfig),
		zapcore.AddSync(&GlobalLoggedBuffer),
		level,
	)
}

// Logger implementation
func (l *Logger) SetVerbose(verbose bool) {
	l.verbose = verbose
}

func (l *Logger) syncIfNeeded() {
	if GlobalInstantSync {
		_ = l.Sync()
	}
}

// Basic logging methods
func (l *Logger) Debug(msg string) {
	formattedMsg := formatMessage(msg)
	l.Logger.Debug(formattedMsg)
	globalLogBuffer.AddLine(formattedMsg)
	l.syncIfNeeded()
}
func (l *Logger) Info(msg string) {
	formattedMsg := formatMessage(msg)
	l.Logger.Info(formattedMsg)
	globalLogBuffer.AddLine(formattedMsg)
	l.syncIfNeeded()
}
func (l *Logger) Warn(msg string) {
	formattedMsg := formatMessage(msg)
	l.Logger.Warn(formattedMsg)
	globalLogBuffer.AddLine(formattedMsg)
	l.syncIfNeeded()
}
func (l *Logger) Error(msg string) {
	formattedMsg := formatMessage(msg)
	l.Logger.Error(formattedMsg)
	globalLogBuffer.AddLine(formattedMsg)
	l.syncIfNeeded()
}
func (l *Logger) Fatal(msg string) {
	formattedMsg := formatMessage(msg)
	l.Logger.Fatal(formattedMsg)
}

// Formatted logging methods
func (l *Logger) Debugf(
	format string,
	args ...interface{},
) {
	l.Debug(fmt.Sprintf(format, args...))
}
func (l *Logger) Infof(format string, args ...interface{}) { l.Info(fmt.Sprintf(format, args...)) }
func (l *Logger) Warnf(format string, args ...interface{}) { l.Warn(fmt.Sprintf(format, args...)) }

func (l *Logger) Errorf(
	format string,
	args ...interface{},
) {
	l.Error(fmt.Sprintf(format, args...))
}

func (l *Logger) Fatalf(
	format string,
	args ...interface{},
) {
	l.Fatal(fmt.Sprintf(format, args...))
}

// Field logging methods
func (l *Logger) DebugWithFields(msg string, fields ...zap.Field) {
	l.Logger.Debug(formatMessage(msg), fields...)
	l.syncIfNeeded()
}
func (l *Logger) InfoWithFields(msg string, fields ...zap.Field) {
	l.Logger.Info(formatMessage(msg), fields...)
	l.syncIfNeeded()
}
func (l *Logger) WarnWithFields(msg string, fields ...zap.Field) {
	l.Logger.Warn(formatMessage(msg), fields...)
	l.syncIfNeeded()
}
func (l *Logger) ErrorWithFields(msg string, fields ...zap.Field) {
	l.Logger.Error(formatMessage(msg), fields...)
	l.syncIfNeeded()
}

// TestLogger implementation
func (tl *TestLogger) GetLogs() []string {
	tl.logLock.Lock()
	defer tl.logLock.Unlock()
	return append([]string{}, *tl.logs...)
}

// Test logger methods with capture
func (tl *TestLogger) Debug(msg string) {
	tl.logLock.Lock()
	*tl.logs = append(*tl.logs, msg)
	tl.logLock.Unlock()
	tl.Logger.Debug(msg)
}

func (tl *TestLogger) Info(msg string) {
	tl.logLock.Lock()
	*tl.logs = append(*tl.logs, msg)
	tl.logLock.Unlock()
	tl.Logger.Info(msg)
}

func (tl *TestLogger) Warn(msg string) {
	tl.logLock.Lock()
	*tl.logs = append(*tl.logs, msg)
	tl.logLock.Unlock()
	tl.Logger.Warn(msg)
}

func (tl *TestLogger) Error(msg string) {
	tl.logLock.Lock()
	*tl.logs = append(*tl.logs, msg)
	tl.logLock.Unlock()
	tl.Logger.Error(msg)
}

// Utility functions
func formatMessage(msg string) string {
	return strings.TrimPrefix(msg, "andaime\t")
}

func customTimeEncoder(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(fmt.Sprintf("[%s]", t.Format("2006-01-02 15:04:05")))
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
		return zapcore.InfoLevel
	}
}

// Global functions
func Get() *Logger {
	loggerMutex.Lock()
	defer loggerMutex.Unlock()

	if globalLogger == nil {
		InitProduction()
	}
	return &Logger{Logger: globalLogger, verbose: false}
}

func SetGlobalLogger(logger interface{}) {
	loggerMutex.Lock()
	defer loggerMutex.Unlock()
	switch l := logger.(type) {
	case *Logger:
		globalLogger = l.Logger
	case *TestLogger:
		globalLogger = l.Logger.Logger
	default:
		panic("unsupported logger type")
	}
}

// Create new loggers
func NewTestLogger(tb zaptest.TestingT) *TestLogger {
	var t *testing.T
	if tt, ok := tb.(*testing.T); ok {
		t = tt
	} else {
		panic("tb does not implement *testing.T")
	}
	core := zapcore.NewCore(
		zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig()),
		zapcore.AddSync(&testingWriter{tb: tb}),
		levelEnabler{},
	)
	logs := make([]string, 0)
	return &TestLogger{
		Logger: &Logger{
			Logger:  zap.New(core),
			verbose: true,
		},
		t:       t,
		logs:    &logs,
		logLock: &sync.Mutex{},
	}
}

func NewNopLogger() *Logger {
	return &Logger{Logger: zap.NewNop(), verbose: false}
}

// Panic handling
func LogPanic(rec interface{}) {
	stack := debug.Stack()
	Get().ErrorWithFields("PANIC", zap.String("stack", string(stack)))
}

func RecoverAndLog(f func()) {
	defer func() {
		if r := recover(); r != nil {
			LogPanic(r)
			panic(r)
		}
	}()
	f()
}

// LogBuffer maintains a circular buffer of log messages
type LogBuffer struct {
	lines []string
	size  int
	mu    sync.RWMutex
}

// NewLogBuffer creates a new log buffer with specified size
func NewLogBuffer(size int) *LogBuffer {
	return &LogBuffer{
		lines: make([]string, 0, size),
		size:  size,
	}
}

var (
	// Global log buffer
	globalLogBuffer = NewLogBuffer(GlobalLoggedBufferSize) // Default size, can be configured
)

// AddLine adds a line to the buffer, maintaining the circular buffer behavior
func (lb *LogBuffer) AddLine(line string) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	if len(lb.lines) >= lb.size {
		// Remove oldest line
		lb.lines = lb.lines[1:]
	}
	lb.lines = append(lb.lines, line)
}

// GetLastLines returns the last n lines from the buffer
func (lb *LogBuffer) GetLastLines(n int) []string {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	if n <= 0 {
		return []string{}
	}

	if n >= len(lb.lines) {
		return append([]string{}, lb.lines...)
	}

	return append([]string{}, lb.lines[len(lb.lines)-n:]...)
}

// GetLastLines gets the last n lines from the log
func GetLastLines(n int) []string {
	return globalLogBuffer.GetLastLines(n)
}

// Update test logger methods to use buffer
func (tl *TestLogger) GetLastLines(n int) []string {
	return tl.buffer.GetLastLines(n)
}

// PrintLogs prints all captured logs to the test output
func (tl *TestLogger) PrintLogs(t *testing.T) {
	tl.logLock.Lock()
	defer tl.logLock.Unlock()

	t.Log("Captured logs:")
	for i, log := range *tl.logs {
		if log != "" {
			t.Logf("[%d] %s", i, log)
		}
	}
}

// PrintLogs prints all captured logs to the test output
func (l *Logger) PrintLogs(t *testing.T) {
	t.Log("Captured logs:")
	for i, log := range globalLogBuffer.GetLastLines(100) {
		if log != "" {
			t.Logf("[%d] %s", i, log)
		}
	}
}

type Loggerer interface {
	Debug(msg string)
	Info(msg string)
	Warn(msg string)
	Error(msg string)
	SetVerbose(bool)
	Debugf(format string, args ...interface{})
	Infof(format string, args ...interface{})
	Warnf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
	PrintLogs(*testing.T)
	With(fields ...zap.Field) Loggerer
}

// Update TestLogger's With method
func (tl *TestLogger) With(fields ...zap.Field) Loggerer {
	return &TestLogger{
		Logger:  tl.Logger.With(fields...).(*Logger),
		t:       tl.t,
		logs:    tl.logs,
		logLock: tl.logLock, // Reuse the existing mutex
	}
}

// Update Logger's With method
func (l *Logger) With(fields ...zap.Field) Loggerer {
	return &Logger{
		Logger:  l.Logger.With(fields...),
		verbose: l.verbose,
	}
}

var _ Loggerer = &TestLogger{}
var _ Loggerer = &Logger{}
