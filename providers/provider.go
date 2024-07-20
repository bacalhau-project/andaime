package providers

type CloudProvider interface {
	CreateCluster(clusterName string, numMachines int) error
	ListResources() error
	DestroyResources() error
}
