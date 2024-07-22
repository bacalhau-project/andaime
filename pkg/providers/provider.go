package providers

type CloudProviderer interface {
	CreateCluster(clusterName string, numMachines int) error
	ListResources() error
	DestroyResources() error
}
