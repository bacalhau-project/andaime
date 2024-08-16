package internal

import (
	"embed"
)

//go:embed 10_install_docker.sh
var installDocker embed.FS

//go:embed 15_install_core_packages.sh
var installCorePackages embed.FS

//go:embed 20_get_node_config_metadata.sh
var getNodeConfigMetadata embed.FS

//go:embed 100_install_bacalhau.sh
var installBacalhau embed.FS

//go:embed 105_install_bacalhau_compute.sh
var installBacalhauCompute embed.FS

//go:embed 110_install_and_restart_bacalhau_service.sh
var installAndRestartBacalhauServices embed.FS
