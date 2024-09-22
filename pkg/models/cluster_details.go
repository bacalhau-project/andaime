package models

// ClusterDetails represents the details of a cluster
type ClusterDetails struct {
	Name              string
	DeploymentType    DeploymentType
	KubernetesVersion string
	NodeCount         int
	NodeTypes         []string
}
