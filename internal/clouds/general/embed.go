package internal

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

//go:embed 105_install_bacalhau_compute.sh
var InstallBacalhauCompute embed.FS

func GetInstallBacalhauComputeScript() ([]byte, error) {
	script, err := InstallBacalhauCompute.ReadFile("105_install_bacalhau_compute.sh")
	if err != nil {
		return nil, err
	}
	return script, nil
}

//go:embed 110_install_and_restart_bacalhau_service.sh
var InstallAndRestartBacalhauServices embed.FS

func GetInstallAndRestartBacalhauServicesScript() ([]byte, error) {
	script, err := InstallAndRestartBacalhauServices.ReadFile(
		"110_install_and_restart_bacalhau_service.sh",
	)
	if err != nil {
		return nil, err
	}
	return script, nil
}
