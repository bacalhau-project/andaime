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
