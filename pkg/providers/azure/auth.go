import (
	"fmt"
	"strings"
)

func wrapAzureError(err error) error {
	if err == nil {
		return nil
	}
	
	errMsg := err.Error()
	if strings.Contains(errMsg, "Failed to establish a new connection") {
		return fmt.Errorf("unable to connect to Azure. Please check your internet connection and try again: %w", err)
	}
	if strings.Contains(errMsg, "Max retries exceeded") {
		return fmt.Errorf("connection to Azure timed out. Please check your network and try again: %w", err)
	}
	if strings.Contains(errMsg, "Azure Developer CLI not found on path") {
		return fmt.Errorf("Azure Developer CLI not found. Please install it and ensure it's in your PATH: %w", err)
	}
	return fmt.Errorf("an error occurred while connecting to Azure: %w", err)
}
