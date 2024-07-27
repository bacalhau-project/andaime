package cmd

import (
	"bytes"

	"github.com/spf13/cobra"
)

func ExecuteCommand(root *cobra.Command, args ...string) (output string, err error) {
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(args)

	defer func() {
		if r := recover(); r != nil {
			logger.Get().Errorf("Panic occurred: %v", r)
			logger.Get().Sync()
			err = fmt.Errorf("panic occurred: %v", r)
		}
	}()

	_, err = root.ExecuteC()

	// Ensure logs are flushed
	logger.Get().Sync()

	return buf.String(), err
}
