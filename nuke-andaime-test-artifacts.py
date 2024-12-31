import time

import boto3
from botocore.exceptions import ClientError


def get_all_regions():
    """Get all AWS regions"""
    ec2 = boto3.client("ec2")
    response = ec2.describe_regions()
    return [region["RegionName"] for region in response["Regions"]]


def find_matching_vpcs(region):
    """Find VPCs with matching tags in a region"""
    ec2 = boto3.client("ec2", region_name=region)
    response = ec2.describe_vpcs(
        Filters=[
            {"Name": "tag:Name", "Values": ["andaime-vpc*"]},
            {"Name": "tag:andaime", "Values": ["true"]},
        ]
    )
    return response["Vpcs"]


def list_vpc_resources(region, vpc_id):
    """List all resources in a VPC"""
    ec2 = boto3.client("ec2", region_name=region)

    print(f"\n=== Resources in VPC {vpc_id} ({region}) ===")

    # List subnets
    subnets = ec2.describe_subnets(Filters=[{"Name": "vpc-id", "Values": [vpc_id]}])
    print("Subnets:")
    for subnet in subnets["Subnets"]:
        print(f"  - {subnet['SubnetId']}")

    # List internet gateways
    igws = ec2.describe_internet_gateways(
        Filters=[{"Name": "attachment.vpc-id", "Values": [vpc_id]}]
    )
    print("Internet Gateways:")
    for igw in igws["InternetGateways"]:
        print(f"  - {igw['InternetGatewayId']}")

    # List route tables
    route_tables = ec2.describe_route_tables(
        Filters=[{"Name": "vpc-id", "Values": [vpc_id]}]
    )
    print("Route Tables:")
    for rt in route_tables["RouteTables"]:
        if not any(assoc["Main"] for assoc in rt["Associations"]):
            print(f"  - {rt['RouteTableId']}")

    # List security groups
    security_groups = ec2.describe_security_groups(
        Filters=[{"Name": "vpc-id", "Values": [vpc_id]}]
    )
    print("Security Groups:")
    for sg in security_groups["SecurityGroups"]:
        if sg["GroupName"] != "default":
            print(f"  - {sg['GroupId']}")

    # List instances
    instances = ec2.describe_instances(Filters=[{"Name": "vpc-id", "Values": [vpc_id]}])
    print("Instances:")
    for reservation in instances["Reservations"]:
        for instance in reservation["Instances"]:
            print(f"  - {instance['InstanceId']}")


def delete_vpc_resources(region, vpc_id):
    """Delete all resources in a VPC"""
    ec2 = boto3.client("ec2", region_name=region)

    # Terminate instances
    instances = ec2.describe_instances(Filters=[{"Name": "vpc-id", "Values": [vpc_id]}])
    instance_ids = [
        i["InstanceId"] for r in instances["Reservations"] for i in r["Instances"]
    ]

    if instance_ids:
        print(f"Terminating instances in VPC {vpc_id}...")
        ec2.terminate_instances(InstanceIds=instance_ids)
        waiter = ec2.get_waiter("instance_terminated")
        waiter.wait(InstanceIds=instance_ids)
        print("All instances terminated")

    # Delete security groups (except default)
    security_groups = ec2.describe_security_groups(
        Filters=[{"Name": "vpc-id", "Values": [vpc_id]}]
    )
    for sg in security_groups["SecurityGroups"]:
        if sg["GroupName"] != "default":
            print(f"Deleting security group {sg['GroupId']}")
            ec2.delete_security_group(GroupId=sg["GroupId"])

    # Delete subnets
    subnets = ec2.describe_subnets(Filters=[{"Name": "vpc-id", "Values": [vpc_id]}])
    for subnet in subnets["Subnets"]:
        print(f"Deleting subnet {subnet['SubnetId']}")
        ec2.delete_subnet(SubnetId=subnet["SubnetId"])

    # Detach and delete internet gateways
    igws = ec2.describe_internet_gateways(
        Filters=[{"Name": "attachment.vpc-id", "Values": [vpc_id]}]
    )
    for igw in igws["InternetGateways"]:
        print(f"Detaching and deleting internet gateway {igw['InternetGatewayId']}")
        ec2.detach_internet_gateway(
            InternetGatewayId=igw["InternetGatewayId"], VpcId=vpc_id
        )
        ec2.delete_internet_gateway(InternetGatewayId=igw["InternetGatewayId"])

    # Delete route tables (except main)
    route_tables = ec2.describe_route_tables(
        Filters=[{"Name": "vpc-id", "Values": [vpc_id]}]
    )
    for rt in route_tables["RouteTables"]:
        if not any(assoc["Main"] for assoc in rt["Associations"]):
            print(f"Deleting route table {rt['RouteTableId']}")
            ec2.delete_route_table(RouteTableId=rt["RouteTableId"])

    # Delete VPC with retries
    max_retries = 3
    for attempt in range(max_retries):
        try:
            print(f"Deleting VPC {vpc_id} (attempt {attempt + 1}/{max_retries})...")
            ec2.delete_vpc(VpcId=vpc_id)
            print(f"VPC {vpc_id} deleted successfully")
            break
        except ClientError as e:
            if attempt == max_retries - 1:
                print(f"Failed to delete VPC {vpc_id} after {max_retries} attempts")
                raise
            print(f"Error deleting VPC, retrying... ({e})")
            time.sleep(5)


def main():
    regions = get_all_regions()

    print("=== Listing all VPCs matching 'andaime-vpc*' ===")
    for region in regions:
        vpcs = find_matching_vpcs(region)
        if not vpcs:
            print(f"No matching VPCs found in region {region}")
            continue

        for vpc in vpcs:
            vpc_id = vpc["VpcId"]
            vpc_name = next(
                (tag["Value"] for tag in vpc.get("Tags", []) if tag["Key"] == "Name"),
                "",
            )
            print(f"\nFound VPC: {vpc_id} ({vpc_name}) in region {region}")
            print("Tags:", {tag["Key"]: tag["Value"] for tag in vpc.get("Tags", [])})
            list_vpc_resources(region, vpc_id)

    confirm = input(
        "\nAre you sure you want to delete all these VPCs and their resources? (y/N) "
    ).lower()
    if confirm != "y":
        print("Aborted.")
        return

    for region in regions:
        vpcs = find_matching_vpcs(region)
        if not vpcs:
            print(f"No matching VPCs found in region {region}")
            continue

        for vpc in vpcs:
            vpc_id = vpc["VpcId"]
            vpc_name = next(
                (tag["Value"] for tag in vpc.get("Tags", []) if tag["Key"] == "Name"),
                "",
            )
            print(f"\nProcessing VPC: {vpc_id} ({vpc_name}) in region {region}")
            print("Tags:", {tag["Key"]: tag["Value"] for tag in vpc.get("Tags", [])})
            try:
                delete_vpc_resources(region, vpc_id)
            except Exception as e:
                print(f"Error processing VPC {vpc_id}: {e}")
                continue

    print("Cleanup complete!")


if __name__ == "__main__":
    main()
