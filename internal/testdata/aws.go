package testdata

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

func FakeEC2DescribeAvailabilityZonesOutput() *ec2.DescribeAvailabilityZonesOutput {
	return &ec2.DescribeAvailabilityZonesOutput{
		AvailabilityZones: []types.AvailabilityZone{
			{
				ZoneName: aws.String("us-east-1a"),
			},
		},
	}
}

func FakeEC2RunInstancesOutput() *ec2.RunInstancesOutput {
	return &ec2.RunInstancesOutput{
		Instances: []types.Instance{
			{
				InstanceId:      aws.String("i-1234567890abcdef0"),
				InstanceType:    types.InstanceTypeT3Medium,
				PublicIpAddress: aws.String("203.0.113.1"),
				SubnetId:        aws.String("subnet-12345"),
				VpcId:           aws.String("vpc-12345"),
				State: &types.InstanceState{
					Name: types.InstanceStateNameRunning,
				},
				Tags: []types.Tag{
					{
						Key:   aws.String("Name"),
						Value: aws.String("test-instance"),
					},
				},
			},
		},
	}
}

func FakeEC2DescribeInstancesOutput() *ec2.DescribeInstancesOutput {
	return &ec2.DescribeInstancesOutput{
		Reservations: []types.Reservation{
			{
				Instances: []types.Instance{
					{
						InstanceId:       aws.String("i-1234567890abcdef0"),
						InstanceType:     types.InstanceTypeT3Medium,
						PublicIpAddress:  aws.String("203.0.113.1"),
						PrivateIpAddress: aws.String("10.0.0.1"),
						SubnetId:         aws.String("subnet-12345"),
						VpcId:            aws.String("vpc-12345"),
						State: &types.InstanceState{
							Name: types.InstanceStateNameRunning,
						},
						Tags: []types.Tag{
							{
								Key:   aws.String("Name"),
								Value: aws.String("test-instance"),
							},
						},
					},
				},
			},
		},
	}
}

func FakeEC2CreateVpcOutput() *ec2.CreateVpcOutput {
	return &ec2.CreateVpcOutput{
		Vpc: &types.Vpc{
			VpcId:     aws.String("vpc-12345"),
			CidrBlock: aws.String("10.0.0.0/16"),
			State:     types.VpcStateAvailable,
		},
	}
}

func FakeEC2DescribeVpcsOutput() *ec2.DescribeVpcsOutput {
	return &ec2.DescribeVpcsOutput{
		Vpcs: []types.Vpc{
			{
				VpcId:     aws.String("vpc-12345"),
				CidrBlock: aws.String("10.0.0.0/16"),
				State:     types.VpcStateAvailable,
			},
		},
	}
}

func FakeEC2CreateSubnetOutput() *ec2.CreateSubnetOutput {
	return &ec2.CreateSubnetOutput{
		Subnet: &types.Subnet{
			SubnetId:  aws.String("subnet-12345"),
			VpcId:     aws.String("vpc-12345"),
			CidrBlock: aws.String("10.0.1.0/24"),
			State:     types.SubnetStateAvailable,
			Tags: []types.Tag{
				{
					Key:   aws.String("Name"),
					Value: aws.String("test-subnet"),
				},
			},
		},
	}
}

func FakeEC2DescribeSubnetsOutput() *ec2.DescribeSubnetsOutput {
	return &ec2.DescribeSubnetsOutput{
		Subnets: []types.Subnet{
			{
				SubnetId:  aws.String("subnet-12345"),
				VpcId:     aws.String("vpc-12345"),
				CidrBlock: aws.String("10.0.1.0/24"),
				State:     types.SubnetStateAvailable,
				Tags: []types.Tag{
					{
						Key:   aws.String("Name"),
						Value: aws.String("test-subnet"),
					},
				},
			},
		},
	}
}

func FakeEC2CreateSecurityGroupOutput() *ec2.CreateSecurityGroupOutput {
	return &ec2.CreateSecurityGroupOutput{
		GroupId: aws.String("sg-1234567890abcdef0"),
	}
}

func FakeEC2DescribeSecurityGroupsOutput() *ec2.DescribeSecurityGroupsOutput {
	return &ec2.DescribeSecurityGroupsOutput{
		SecurityGroups: []types.SecurityGroup{
			{
				GroupId: aws.String("sg-1234567890abcdef0"),
			},
		},
	}
}

func FakeEC2CreateInternetGatewayOutput() *ec2.CreateInternetGatewayOutput {
	return &ec2.CreateInternetGatewayOutput{
		InternetGateway: &types.InternetGateway{
			InternetGatewayId: aws.String("igw-12345"),
			Tags: []types.Tag{
				{
					Key:   aws.String("Name"),
					Value: aws.String("test-igw"),
				},
			},
		},
	}
}

func FakeEC2AttachInternetGatewayOutput() *ec2.AttachInternetGatewayOutput {
	return &ec2.AttachInternetGatewayOutput{}
}

func FakeEC2CreateRouteTableOutput() *ec2.CreateRouteTableOutput {
	return &ec2.CreateRouteTableOutput{
		RouteTable: &types.RouteTable{
			RouteTableId: aws.String("rtb-12345"),
			VpcId:        aws.String("vpc-12345"),
			Routes: []types.Route{
				{
					DestinationCidrBlock: aws.String("0.0.0.0/0"),
					GatewayId:            aws.String("igw-12345"),
					State:                types.RouteStateActive,
				},
			},
		},
	}
}

func FakeEC2CreateRouteOutput() *ec2.CreateRouteOutput {
	return &ec2.CreateRouteOutput{
		Return: aws.Bool(true),
	}
}

func FakeEC2AssociateRouteTableOutput() *ec2.AssociateRouteTableOutput {
	return &ec2.AssociateRouteTableOutput{
		AssociationId: aws.String("rtbassoc-12345"),
	}
}

func FakeEC2DescribeRouteTablesOutput() *ec2.DescribeRouteTablesOutput {
	return &ec2.DescribeRouteTablesOutput{
		RouteTables: []types.RouteTable{
			{
				RouteTableId: aws.String("rtb-12345"),
				VpcId:        aws.String("vpc-12345"),
				Routes: []types.Route{
					{
						DestinationCidrBlock: aws.String("0.0.0.0/0"),
						GatewayId:            aws.String("igw-12345"),
						State:                types.RouteStateActive,
					},
				},
				Associations: []types.RouteTableAssociation{
					{
						RouteTableId:            aws.String("rtb-12345"),
						RouteTableAssociationId: aws.String("rtbassoc-12345"),
						Main:                    aws.Bool(true),
					},
				},
			},
		},
	}
}