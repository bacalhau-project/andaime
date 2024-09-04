package internal_gcp

import (
	"embed"
)

//go:embed gcp_data.yaml
var gcpData embed.FS

//go:embed startup_script.sh
var startupScript embed.FS

func GetStartupScript() (string, error) {
	script, err := startupScript.ReadFile("startup_script.sh")
	if err != nil {
		return "", err
	}

	return string(script), nil
}

func GetGCPData() ([]byte, error) {
	data, err := gcpData.ReadFile("gcp_data.yaml")
	if err != nil {
		return nil, err
	}
	return data, nil
}
