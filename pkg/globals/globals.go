package globals

type contextKey string

const UniqueDeploymentIDKey contextKey = "UniqueDeploymentID"
const MillisecondsBetweenUpdates = 100
const NumberOfSecondsToProbeResourceGroup = 2
const DefaultDiskSizeGB = 30

var (
	DefaultAllowedPorts = []int{22, 80, 443}
)
