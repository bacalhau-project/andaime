package providers

import "time"

const (
	BackoffTime = time.Duration(2) * time.Second
)

type CloudProviderer interface {
	CreateCluster(clusterName string, numMachines int) error
	ListResources() error
	DestroyResources() error
}
