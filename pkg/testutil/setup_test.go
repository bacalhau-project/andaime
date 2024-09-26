package testutil

import (
	"os"
	"testing"

	"github.com/bacalhau-project/andaime/pkg/logger"
)

func TestMain(m *testing.M) {
	// Initialize logger before running any tests
	logger.InitProduction()

	// Run tests
	code := m.Run()

	// Exit with the test result code
	os.Exit(code)
}

// SetupTest is a helper function to be called at the beginning of each test
func SetupTest(t *testing.T) {
	t.Helper()
}
