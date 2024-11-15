package logger

import (
	"sync"
	"testing"
)

// TestLogger extends Logger with log capture for testing
type TestLogger struct {
	*Logger
	logs    []string
	t       *testing.T
	logLock sync.Mutex
}

// NewTestLogger creates a new test logger
func NewTestLogger(t *testing.T) *TestLogger {
	return &TestLogger{
		Logger: NewLogger(&LoggerConfig{
			Console: false,
			File:    false,
			Buffer:  true,
		}),
		t:    t,
		logs: make([]string, 0),
	}
}

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
