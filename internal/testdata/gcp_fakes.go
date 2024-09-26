package testdata

import (
	"cloud.google.com/go/compute/apiv1/computepb"
	"google.golang.org/protobuf/proto"
)

// FakeGCPInstance returns a fake GCP Virtual Machine for testing
func FakeGCPInstance() *computepb.Instance {
	return &computepb.Instance{
		Name: proto.String("fake-instance"),
		MachineType: proto.String(
			"https://www.googleapis.com/compute/v1/projects/fake-project/zones/fake-zone/machineTypes/n1-standard-2",
		),
		Disks: []*computepb.AttachedDisk{
			{
				Source: proto.String(
					"https://www.googleapis.com/compute/v1/projects/fake-project/zones/fake-zone/disks/fake-disk",
				),
				Type: proto.String(computepb.AttachedDisk_PERSISTENT.String()),
			},
		},
		NetworkInterfaces: []*computepb.NetworkInterface{
			{
				AccessConfigs: []*computepb.AccessConfig{
					{
						NatIP: proto.String("192.168.1.1"),
					},
				},
				NetworkIP: proto.String("10.0.1.1"),
			},
		},
	}
}
package testdata

import (
	"github.com/bacalhau-project/andaime/pkg/models/interfaces/gcp"
)

// FakeGCPInstance returns a fake GCP Instance for testing
func FakeGCPInstance() *gcp.Instance {
	return &gcp.Instance{
		Name: "test-instance",
		NetworkInterfaces: []*gcp.NetworkInterface{
			{
				NetworkIP: "10.0.0.2",
				AccessConfigs: []*gcp.AccessConfig{
					{
						NatIP: "35.200.100.100",
					},
				},
			},
		},
		MachineType: "projects/test-project/zones/us-central1-a/machineTypes/n1-standard-1",
		Zone:        "projects/test-project/zones/us-central1-a",
	}
}

// FakeGCPProject returns a fake GCP Project for testing
func FakeGCPProject() *gcp.Project {
	return &gcp.Project{
		Name:      "test-project",
		ProjectId: "test-project-id",
	}
}

// FakeGCPOperation returns a fake GCP Operation for testing
func FakeGCPOperation() *gcp.Operation {
	return &gcp.Operation{
		Name:   "test-operation",
		Status: "DONE",
		Zone:   "projects/test-project/zones/us-central1-a",
	}
}

// FakeGCPMachineType returns a fake GCP Machine Type for testing
func FakeGCPMachineType() *gcp.MachineType {
	return &gcp.MachineType{
		Name:        "n1-standard-1",
		Description: "1 vCPU, 3.75 GB RAM",
		GuestCpus:   1,
		MemoryMb:    3840,
	}
}

// FakeGCPNetwork returns a fake GCP Network for testing
func FakeGCPNetwork() *gcp.Network {
	return &gcp.Network{
		Name:                  "test-network",
		AutoCreateSubnetworks: true,
	}
}

// FakeGCPSubnetwork returns a fake GCP Subnetwork for testing
func FakeGCPSubnetwork() *gcp.Subnetwork {
	return &gcp.Subnetwork{
		Name:        "test-subnetwork",
		IpCidrRange: "10.0.0.0/24",
		Network:     "projects/test-project/global/networks/test-network",
	}
}
