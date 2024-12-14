#!/bin/bash

# Parse command line arguments
DRY_RUN=false
FORCE=false
MAX_RETRIES=3

print_usage() {
    echo "Usage: $0 [-d|--dry-run] [-f|--force]"
    echo "  -d, --dry-run    Run in dry-run mode (no actual deletions)"
    echo "  -f, --force      Force deletion without confirmation"
    exit 1
}

while [[ "$#" -gt 0 ]]; do
    case $1 in
        -d|--dry-run) DRY_RUN=true ;;
        -f|--force) FORCE=true ;;
        -h|--help) print_usage ;;
        *) echo "Unknown parameter: $1"; print_usage ;;
    esac
    shift
done

if $DRY_RUN; then
    echo "Running in DRY-RUN mode - no resources will be deleted"
fi

# Function to execute AWS commands with dry-run support and retries
execute_aws_command() {
    local cmd="$1"
    local retries=0
    
    while [ $retries -lt $MAX_RETRIES ]; do
        if $DRY_RUN; then
            echo "[DRY-RUN] Would execute: $cmd"
            return 0
        else
            cmd="aws --no-cli-pager ${cmd#aws }"
            output=$(eval "$cmd" 2>&1)
            if [ $? -eq 0 ]; then
                echo "$output"
                return 0
            elif [[ $output == *"DependencyViolation"* ]] || [[ $output == *"InvalidParameterValue"* ]]; then
                retries=$((retries + 1))
                if [ $retries -lt $MAX_RETRIES ]; then
                    echo "Dependency error, retrying in 10 seconds... (attempt $retries of $MAX_RETRIES)"
                    sleep 10
                else
                    echo "Failed after $MAX_RETRIES attempts: $output"
                    return 1
                fi
            else
                echo "$output"
                return 1
            fi
        fi
    done
}

# Function to wait for EC2 instances to terminate
wait_for_instances() {
    local region="$1"
    local instance_ids="$2"
    local max_wait=60  # Maximum wait time in seconds
    local waited=0
    
    if [ -z "$instance_ids" ]; then
        return 0
    fi
    
    echo "[$region] Waiting for instances to terminate: $instance_ids"
    while [ $waited -lt $max_wait ]; do
        local status=$(aws --no-cli-pager ec2 describe-instances \
            --region "$region" \
            --instance-ids $instance_ids \
            --query 'Reservations[].Instances[].State.Name' \
            --output text 2>/dev/null || echo "terminated")
        
        if [[ "$status" == "terminated" ]] || [ -z "$status" ]; then
            echo "[$region] All instances terminated"
            return 0
        fi
        
        echo "[$region] Instances still terminating... ($waited seconds)"
        sleep 10
        waited=$((waited + 10))
    done
    
    echo "[$region] Warning: Timed out waiting for instances to terminate"
    return 1
}

# Function to delete a VPC and its resources
delete_vpc() {
    local region="$1"
    local vpc_id="$2"
    
    echo "[$region] Processing VPC: $vpc_id"
    
    if ! $FORCE; then
        read -p "[$region] Are you sure you want to delete VPC $vpc_id and all its resources? (y/N) " confirm
        if [[ $confirm != [yY] ]]; then
            echo "[$region] Skipping VPC $vpc_id"
            return
        fi
    fi

    # 1. Terminate all EC2 instances first and wait
    instance_ids=$(aws --no-cli-pager ec2 describe-instances \
        --region $region \
        --filters "Name=vpc-id,Values=$vpc_id" "Name=instance-state-name,Values=running,stopped,pending,stopping" \
        --query 'Reservations[].Instances[].InstanceId' \
        --output text)
    
    if [ ! -z "$instance_ids" ]; then
        echo "[$region] Terminating EC2 instances: $instance_ids"
        execute_aws_command "aws ec2 terminate-instances --region $region --instance-ids $instance_ids"
        wait_for_instances "$region" "$instance_ids"
    fi

    # 2. Delete ELBs
    elb_names=$(aws --no-cli-pager elb describe-load-balancers \
        --region $region \
        --query "LoadBalancerDescriptions[?VPCId=='$vpc_id'].LoadBalancerName" \
        --output text)
    
    for elb in $elb_names; do
        echo "[$region] Deleting ELB: $elb"
        execute_aws_command "aws elb delete-load-balancer --region $region --load-balancer-name $elb"
    done

    # 3. Delete ALBs/NLBs
    alb_arns=$(aws --no-cli-pager elbv2 describe-load-balancers \
        --region $region \
        --query "LoadBalancers[?VpcId=='$vpc_id'].LoadBalancerArn" \
        --output text)
    
    for alb in $alb_arns; do
        echo "[$region] Deleting ALB/NLB: $alb"
        execute_aws_command "aws elbv2 delete-load-balancer --region $region --load-balancer-arn $alb"
    done

    # 4. Delete NAT Gateways
    nat_gateway_ids=$(aws --no-cli-pager ec2 describe-nat-gateways \
        --region $region \
        --filter "Name=vpc-id,Values=$vpc_id" \
        --query 'NatGateways[*].NatGatewayId' \
        --output text)
    
    for nat_id in $nat_gateway_ids; do
        echo "[$region] Deleting NAT Gateway: $nat_id"
        execute_aws_command "aws ec2 delete-nat-gateway --region $region --nat-gateway-id $nat_id"
    done

    # 5. Delete Network Interfaces
    eni_ids=$(aws --no-cli-pager ec2 describe-network-interfaces \
        --region $region \
        --filters "Name=vpc-id,Values=$vpc_id" \
        --query 'NetworkInterfaces[*].NetworkInterfaceId' \
        --output text)
    
    for eni_id in $eni_ids; do
        echo "[$region] Deleting Network Interface: $eni_id"
        execute_aws_command "aws ec2 delete-network-interface --region $region --network-interface-id $eni_id"
    done

    # 6. Detach and delete Internet Gateways
    igw_ids=$(aws --no-cli-pager ec2 describe-internet-gateways \
        --region $region \
        --filters "Name=attachment.vpc-id,Values=$vpc_id" \
        --query 'InternetGateways[*].InternetGatewayId' \
        --output text)
    
    for igw_id in $igw_ids; do
        echo "[$region] Detaching Internet Gateway: $igw_id"
        execute_aws_command "aws ec2 detach-internet-gateway --region $region --internet-gateway-id $igw_id --vpc-id $vpc_id"
        echo "[$region] Deleting Internet Gateway: $igw_id"
        execute_aws_command "aws ec2 delete-internet-gateway --region $region --internet-gateway-id $igw_id"
    done

    # 7. Delete Route Table associations and Routes
    rt_ids=$(aws --no-cli-pager ec2 describe-route-tables \
        --region $region \
        --filters "Name=vpc-id,Values=$vpc_id" \
        --query 'RouteTables[?Associations[0].Main != `true`].RouteTableId' \
        --output text)
    
    for rt_id in $rt_ids; do
        # Delete non-main route table associations
        assoc_ids=$(aws --no-cli-pager ec2 describe-route-tables \
            --region $region \
            --route-table-ids $rt_id \
            --query 'RouteTables[*].Associations[?!Main].RouteTableAssociationId' \
            --output text)
        
        for assoc_id in $assoc_ids; do
            echo "[$region] Deleting Route Table association: $assoc_id"
            execute_aws_command "aws ec2 disassociate-route-table --region $region --association-id $assoc_id"
        done

        # Delete routes
        echo "[$region] Deleting routes from Route Table: $rt_id"
        execute_aws_command "aws ec2 delete-route --region $region --route-table-id $rt_id --destination-cidr-block 0.0.0.0/0"
    done

    # 8. Delete Subnets
    subnet_ids=$(aws --no-cli-pager ec2 describe-subnets \
        --region $region \
        --filters "Name=vpc-id,Values=$vpc_id" \
        --query 'Subnets[*].SubnetId' \
        --output text)
    
    for subnet_id in $subnet_ids; do
        echo "[$region] Deleting Subnet: $subnet_id"
        execute_aws_command "aws ec2 delete-subnet --region $region --subnet-id $subnet_id"
    done

    # 9. Delete Route Tables
    for rt_id in $rt_ids; do
        echo "[$region] Deleting Route Table: $rt_id"
        execute_aws_command "aws ec2 delete-route-table --region $region --route-table-id $rt_id"
    done

    # 10. Delete Security Groups
    sg_ids=$(aws --no-cli-pager ec2 describe-security-groups \
        --region $region \
        --filters "Name=vpc-id,Values=$vpc_id" \
        --query 'SecurityGroups[?GroupName != `default`].GroupId' \
        --output text)
    
    for sg_id in $sg_ids; do
        echo "[$region] Deleting Security Group: $sg_id"
        execute_aws_command "aws ec2 delete-security-group --region $region --group-id $sg_id"
    done

    # 11. Delete the VPC
    echo "[$region] Deleting VPC: $vpc_id"
    execute_aws_command "aws ec2 delete-vpc --region $region --vpc-id $vpc_id"
    
    echo "[$region] Initiated deletion of VPC $vpc_id and its dependencies"
}

# Export variables and functions for parallel execution
export -f delete_vpc execute_aws_command wait_for_instances
export DRY_RUN FORCE MAX_RETRIES

# Get list of all AWS regions
regions=$(aws --no-cli-pager ec2 describe-regions --query 'Regions[*].RegionName' --output text)

# Create a temporary file to store region/VPC pairs
temp_file=$(mktemp)
trap 'rm -f $temp_file' EXIT

# Collect all region/VPC pairs
for region in $regions; do
    echo "Checking region: $region"
    
    # Get VPCs with either the Name tag or andaime tag
    vpc_ids=$(aws --no-cli-pager ec2 describe-vpcs \
        --region $region \
        --filters \
        "Name=tag:andaime,Values=true" \
        --query 'Vpcs[*].VpcId' \
        --output text)
    
    if [ ! -z "$vpc_ids" ]; then
        for vpc_id in $vpc_ids; do
            echo "$region $vpc_id" >> $temp_file
        done
    fi
done

# If no VPCs found, exit
if [ ! -s $temp_file ]; then
    echo "No VPCs found to delete"
    exit 0
fi

echo "Found the following VPCs to delete:"
cat $temp_file

if ! $FORCE; then
    read -p "Do you want to proceed with deletion? (y/N) " confirm
    if [[ $confirm != [yY] ]]; then
        echo "Aborting deletion"
        exit 0
    fi
fi

# Process all regions in parallel using xargs
echo "Starting parallel deletion across regions..."
cat $temp_file | xargs -P 8 -L 1 bash -c 'delete_vpc "$0" "$1"'

echo "VPC deletion process initiated across all regions"
