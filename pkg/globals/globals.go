package globals

import "time"

type contextKey string

const UniqueDeploymentIDKey contextKey = "UniqueDeploymentID"
const MillisecondsBetweenUpdates = 100
const NumberOfSecondsToProbeResourceGroup = 2
const DefaultDiskSizeGB = 30

const WaitForIPAddressesTimeout = 20 * time.Second
const WaitForResourcesTimeout = 2 * time.Minute
const WaitForResourcesTicker = 5 * time.Second
const MaximumSimultaneousDeployments = 5

const DefaultDiskSize = 30

var (
	DefaultAllowedPorts = []int{22, 80, 443}
)
