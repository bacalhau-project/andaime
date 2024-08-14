package azure

import (
	"strings"
)

func ConvertFromStringToResourceState(state string) (string, error) {
	switch strings.ToLower(state) {
	case "succeeded", "failed", "provisioning":
		return state, nil
	}

	return "not started", nil
}
