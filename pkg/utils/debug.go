package utils

import (
	"fmt"
	"runtime"
	"runtime/pprof"

	"github.com/bacalhau-project/andaime/pkg/logger"
)

func DumpGoroutines() {
	l := logger.Get()
	l.Debug("Dumping goroutines")

	// Get the number of goroutines
	numGoroutines := runtime.NumGoroutine()
	l.Debugf("Number of goroutines: %d", numGoroutines)

	// Dump goroutine information
	_, _ = fmt.Fprintf(&logger.GlobalLoggedBuffer, "Goroutine dump:\n")
	_ = pprof.Lookup("goroutine").WriteTo(&logger.GlobalLoggedBuffer, 1)

	// Log stack traces for all goroutines
	const debugBufferSize = 1 << 20
	buf := make([]byte, debugBufferSize)
	stackLen := runtime.Stack(buf, true)
	l.Debugf("Full goroutine dump:\n%s", buf[:stackLen])
}
