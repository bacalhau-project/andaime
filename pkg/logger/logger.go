package logger

import (
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Global configuration variables
var (
	GlobalEnableConsoleLogger bool
	GlobalEnableFileLogger    bool
	GlobalEnableBufferLogger  bool
	GlobalInstantSync        bool
	GlobalLoggedBufferSize   = 1000 // Default buffer size
	GlobalLogPath           string
	GlobalLogLevel          = "info"
	GlobalLogFile          *os.File
	GlobalLogBuffer        LogBuffer // Exported for use by other packages
	LogFilePermissions = os.FileMode(0644)
)

type Logger interface {
	Debug(args ...interface{})
	Debugf(template string, args ...interface{})
	Info(args ...interface{})
	Infof(template string, args ...interface{})
	Warn(args ...interface{})
	Warnf(template string, args ...interface{})
	Error(args ...interface{})
	Errorf(template string, args ...interface{})
	Fatal(args ...interface{})
	Fatalf(template string, args ...interface{})
	GetLogs() []string
}

// ZapLogger wraps zap.SugaredLogger to implement our Logger interface
type ZapLogger struct {
	*zap.SugaredLogger
}

func (l *ZapLogger) GetLogs() []string {
	return []string{} // Zap logger doesn't store logs
}

func (l *ZapLogger) Fatal(args ...interface{}) {
	l.SugaredLogger.Fatal(args...)
}

func (l *ZapLogger) Fatalf(template string, args ...interface{}) {
	l.SugaredLogger.Fatalf(template, args...)
}

type TestLogger struct {
	mu    sync.Mutex
	logs  []string
	level zapcore.Level
	t     *testing.T
}

func NewTestLogger(t *testing.T) *TestLogger {
	return &TestLogger{
		logs:  make([]string, 0),
		level: zapcore.DebugLevel,
		t:     t,
	}
}

func (l *TestLogger) log(level zapcore.Level, msg string) {
	if level < l.level {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.logs = append(l.logs, msg)
	if l.t != nil {
		l.t.Log(msg)
	}
}

func (l *TestLogger) GetLogs() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string{}, l.logs...)
}

func (l *TestLogger) Debug(args ...interface{}) {
	l.log(zapcore.DebugLevel, fmt.Sprint(args...))
}

func (l *TestLogger) Debugf(template string, args ...interface{}) {
	l.log(zapcore.DebugLevel, fmt.Sprintf(template, args...))
}

func (l *TestLogger) Info(args ...interface{}) {
	l.log(zapcore.InfoLevel, fmt.Sprint(args...))
}

func (l *TestLogger) Infof(template string, args ...interface{}) {
	l.log(zapcore.InfoLevel, fmt.Sprintf(template, args...))
}

func (l *TestLogger) Warn(args ...interface{}) {
	l.log(zapcore.WarnLevel, fmt.Sprint(args...))
}

func (l *TestLogger) Warnf(template string, args ...interface{}) {
	l.log(zapcore.WarnLevel, fmt.Sprintf(template, args...))
}

func (l *TestLogger) Error(args ...interface{}) {
	l.log(zapcore.ErrorLevel, fmt.Sprint(args...))
}

func (l *TestLogger) Errorf(template string, args ...interface{}) {
	l.log(zapcore.ErrorLevel, fmt.Sprintf(template, args...))
}

func (l *TestLogger) Fatal(args ...interface{}) {
	l.log(zapcore.FatalLevel, fmt.Sprint(args...))
}

func (l *TestLogger) Fatalf(template string, args ...interface{}) {
	l.log(zapcore.FatalLevel, fmt.Sprintf(template, args...))
}

var (
	globalLogger Logger
	globalMu     sync.RWMutex
)

func SetGlobalLogger(l Logger) {
	globalMu.Lock()
	defer globalMu.Unlock()
	globalLogger = l
}

func Get() Logger {
	globalMu.RLock()
	defer globalMu.RUnlock()
	if globalLogger == nil {
		config := zap.NewDevelopmentConfig()
		logger, _ := config.Build()
		globalLogger = &ZapLogger{logger.Sugar()}
	}
	return globalLogger
}

type LogBuffer struct {
	lines []string
	size  int
	mu    sync.RWMutex
}

func (lb *LogBuffer) Write(p []byte) (n int, err error) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	if len(lb.lines) >= lb.size {
		lb.lines = lb.lines[1:]
	}
	lb.lines = append(lb.lines, string(p))
	return len(p), nil
}

// GetLastLines returns the last n lines from the buffer
func GetLastLines(n int) []string {
	GlobalLogBuffer.mu.RLock()
	defer GlobalLogBuffer.mu.RUnlock()

	lines := GlobalLogBuffer.lines
	if n <= 0 || len(lines) == 0 {
		return []string{}
	}
	if n > len(lines) {
		n = len(lines)
	}
	return append([]string{}, lines[len(lines)-n:]...)
}

func createBufferCore(level zapcore.Level) zapcore.Core {
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
		zapcore.AddSync(&GlobalLogBuffer),
		level,
	)
}

func customTimeEncoder(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(fmt.Sprintf("[%s]", t.Format("2006-01-02 15:04:05")))
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

func init() {
	GlobalLogBuffer = LogBuffer{
		lines: make([]string, 0),
		size:  GlobalLoggedBufferSize,
	}
}
