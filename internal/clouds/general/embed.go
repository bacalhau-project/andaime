package general

import (
	"embed"
)

//go:embed 10_install_docker.sh
var InstallDockerScript embed.FS

func GetInstallDockerScript() ([]byte, error) {
	script, err := InstallDockerScript.ReadFile("10_install_docker.sh")
	if err != nil {
		return nil, err
	}
	return script, nil
}

//go:embed 15_install_core_packages.sh
var InstallCorePackages embed.FS

func GetInstallCorePackagesScript() ([]byte, error) {
	script, err := InstallCorePackages.ReadFile("15_install_core_packages.sh")
	if err != nil {
		return nil, err
	}
	return script, nil
}

//go:embed 20_get_node_config_metadata.sh
var GetNodeConfigMetadata embed.FS

func GetGetNodeConfigMetadataScript() ([]byte, error) {
	script, err := GetNodeConfigMetadata.ReadFile("20_get_node_config_metadata.sh")
	if err != nil {
		return nil, err
	}
	return script, nil
}

//go:embed 100_install_bacalhau.sh
var InstallBacalhau embed.FS

func GetInstallBacalhauScript() ([]byte, error) {
	script, err := InstallBacalhau.ReadFile("100_install_bacalhau.sh")
	if err != nil {
		return nil, err
	}
	return script, nil
}

//go:embed 105_install_run_bacalhau.sh
var InstallRunBacalhau embed.FS

func GetInstallRunBacalhauScript() ([]byte, error) {
	script, err := InstallRunBacalhau.ReadFile("105_install_run_bacalhau.sh")
	if err != nil {
		return nil, err
	}
	return script, nil
}

//go:embed bacalhau.service
var BacalhauService embed.FS

func GetBacalhauServiceScript() ([]byte, error) {
	script, err := BacalhauService.ReadFile("bacalhau.service")
	if err != nil {
		return nil, err
	}
	return script, nil
}

//go:embed local_custom_script.sh
var LocalCustomScript embed.FS

func GetLocalCustomScript() ([]byte, error) {
	script, err := LocalCustomScript.ReadFile("local_custom_script.sh")
	if err != nil {
		return nil, err
	}
	return script, nil
}

//go:embed bacalhau_settings.json
var BacalhauSettings embed.FS

func GetBacalhauSettings() ([]byte, error) {
	script, err := BacalhauSettings.ReadFile("bacalhau_settings.json")
	if err != nil {
		return nil, err
	}
	return script, nil
}
