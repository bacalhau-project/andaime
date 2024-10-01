package testdata

import (
	"cloud.google.com/go/compute/apiv1/computepb"
	"google.golang.org/protobuf/proto"
)

// FakeGCPInstance returns a fake GCP Instance for testing
func FakeGCPInstance() *computepb.Instance {
	return &computepb.Instance{
		Name: proto.String("test-instance"),
		NetworkInterfaces: []*computepb.NetworkInterface{
			{
				NetworkIP: proto.String("10.0.0.2"),
				AccessConfigs: []*computepb.AccessConfig{
					{
						NatIP: proto.String("35.200.100.100"),
					},
				},
			},
		},
		MachineType: proto.String(
			"projects/test-project/zones/us-central1-a/machineTypes/n1-standard-1",
		),
		Zone: proto.String("projects/test-project/zones/us-central1-a"),
	}
}

// FakeGCPProject returns a fake GCP Project for testing
func FakeGCPProject() *computepb.Project {
	return &computepb.Project{
		Name: proto.String("test-project"),
		Id:   proto.Uint64(1234567890), //nolint:mnd
	}
}

// FakeGCPOperation returns a fake GCP Operation for testing
func FakeGCPOperation() *computepb.Operation {
	status := computepb.Operation_DONE
	return &computepb.Operation{
		Name:   proto.String("test-operation"),
		Zone:   proto.String("projects/test-project/zones/us-central1-a"),
		Status: &status,
	}
}

// FakeGCPMachineType returns a fake GCP Machine Type for testing
func FakeGCPMachineType() *computepb.MachineType {
	return &computepb.MachineType{
		Name:        proto.String("n1-standard-1"),
		Description: proto.String("1 vCPU, 3.75 GB RAM"),
		GuestCpus:   proto.Int32(1),
		MemoryMb:    proto.Int32(3840), //nolint:mnd
	}
}

// FakeGCPNetwork returns a fake GCP Network for testing
func FakeGCPNetwork() *computepb.Network {
	return &computepb.Network{
		Name:                  proto.String("test-network"),
		AutoCreateSubnetworks: proto.Bool(true),
	}
}

// FakeGCPSubnetwork returns a fake GCP Subnetwork for testing
func FakeGCPSubnetwork() *computepb.Subnetwork {
	return &computepb.Subnetwork{
		Name:        proto.String("test-subnetwork"),
		IpCidrRange: proto.String("10.0.0.0/24"),
		Network:     proto.String("projects/test-project/global/networks/test-network"),
	}
}
