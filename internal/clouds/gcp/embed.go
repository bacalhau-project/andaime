package gcp

import (
	"embed"
)

//go:embed startup_script.sh
var startupScript embed.FS

func GetStartupScript() (string, error) {
	script, err := startupScript.ReadFile("startup_script.sh")
	if err != nil {
		return "", err
	}

	return string(script), nil
}
