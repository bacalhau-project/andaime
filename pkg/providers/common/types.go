package common

import (
	"context"

	"github.com/bacalhau-project/andaime/pkg/models"
)

type Providerer interface {
	PrepareDeployment(ctx context.Context) (*models.Deployment, error)
	StartResourcePolling(ctx context.Context)
	CreateResources(ctx context.Context) error
	FinalizeDeployment(ctx context.Context) error
}

type RawMachineParams struct {
	Count           int    `yaml:"count,omitempty"`
	Type            string `yaml:"type,omitempty"`
	Orchestrator    bool   `yaml:"orchestrator,omitempty"`
	DiskSizeGB      int    `yaml:"disk_size_gb,omitempty"`
	DiskImageURL    string `yaml:"disk_image_url,omitempty"`
	DiskImageFamily string `yaml:"disk_image_family,omitempty"`
}

type RawMachine struct {
	Location   string           `yaml:"location"`
	Parameters RawMachineParams `yaml:"parameters,omitempty"`
}
