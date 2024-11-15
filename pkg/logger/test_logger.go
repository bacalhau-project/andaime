package logger

import (
	"sync"
	"testing"
)

// LoggerWithCapture defines the interface for loggers that can capture logs
type LoggerWithCapture interface {
	GetLogs() []string
	PrintLogs()
}

// TestLogger extends Logger with log capture for testing
type TestLogger struct {
	*Logger
	logs    []string
	t       *testing.T 
	logLock sync.Mutex
}

var _ LoggerWithCapture = (*TestLogger)(nil) // Ensure TestLogger implements LoggerWithCapture

// GetLogs returns captured logs
func (tl *TestLogger) GetLogs() []string {
	tl.logLock.Lock()
	defer tl.logLock.Unlock()
	return append([]string{}, tl.logs...)
}

// Override logging methods to capture logs
func (tl *TestLogger) Debug(msg string) {
	tl.logLock.Lock()
	tl.logs = append(tl.logs, msg)
	tl.logLock.Unlock()
	tl.Logger.Debug(msg)
}

func (tl *TestLogger) Info(msg string) {
	tl.logLock.Lock()
	tl.logs = append(tl.logs, msg)
	tl.logLock.Unlock()
	tl.Logger.Info(msg)
}

func (tl *TestLogger) Warn(msg string) {
	tl.logLock.Lock()
	tl.logs = append(tl.logs, msg)
	tl.logLock.Unlock()
	tl.Logger.Warn(msg)
}

func (tl *TestLogger) Error(msg string) {
	tl.logLock.Lock()
	tl.logs = append(tl.logs, msg)
	tl.logLock.Unlock()
	tl.Logger.Error(msg)
}

// PrintLogs prints all captured logs to test output
func (tl *TestLogger) PrintLogs() {
	tl.logLock.Lock()
	defer tl.logLock.Unlock()
	tl.t.Log("Captured logs:")
	for i, log := range tl.logs {
		tl.t.Logf("[%d] %s", i, log)
	}
}
