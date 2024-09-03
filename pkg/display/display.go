package display

import (
	"time"

	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/briandowns/spinner"
)

// NewSpinner creates a new spinner to alert the user about the progress
func NewSpinner(message string) *spinner.Spinner {
	l := logger.Get()
	l.Debugf("Creating spinner: %s", message)

	s := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	s.Prefix = message + " "
	s.Color("green")
	s.Start()

	return s
}
