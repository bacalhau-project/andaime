#!/bin/bash

# Parse command line arguments
DRY_RUN=false

print_usage() {
    echo "Usage: $0 [-d|--dry-run]"
    echo "  -d, --dry-run    Run in dry-run mode (no actual deletions)"
    exit 1
}

while [[ "$#" -gt 0 ]]; do
    case $1 in
        -d|--dry-run) DRY_RUN=true ;;
        -h|--help) print_usage ;;
        *) echo "Unknown parameter: $1"; print_usage ;;
    esac
    shift
done

if $DRY_RUN; then
    echo "Running in DRY-RUN mode - no resources will be deleted"
fi

# Function to execute AWS commands with dry-run support
execute_aws_command() {
    local cmd="$1"
    if $DRY_RUN; then
        echo "[DRY-RUN] Would execute: $cmd"
    else
        eval "$cmd"
    fi
}

# Get list of all AWS regions
regions=$(aws ec2 describe-regions --query 'Regions[*].RegionName' --output text)

# Loop through each region
for region in $regions; do
    echo "Checking region: $region"
    
    # Get VPC IDs for VPCs named "andaime-vpc"
    vpc_ids=$(aws ec2 describe-vpcs \
        --region $region \
        --filters "Name=tag:Name,Values=andaime-vpc" \
        --query 'Vpcs[*].VpcId' \
        --output text)
    
    # If no VPCs found, continue to next region
    if [ -z "$vpc_ids" ]; then
        echo "No matching VPCs found in $region"
        continue
    fi
    
    # Process each VPC
    for vpc_id in $vpc_ids; do
        echo "Processing VPC: $vpc_id in region $region"
        
        # Delete all dependent resources first
        
        # 1. Delete NAT Gateways
        nat_gateway_ids=$(aws ec2 describe-nat-gateways \
            --region $region \
            --filter "Name=vpc-id,Values=$vpc_id" \
            --query 'NatGateways[*].NatGatewayId' \
            --output text)
        
        for nat_id in $nat_gateway_ids; do
            echo "Deleting NAT Gateway: $nat_id"
            execute_aws_command "aws ec2 delete-nat-gateway --region $region --nat-gateway-id $nat_id"
        done
        
        # 2. Delete Internet Gateways
        igw_ids=$(aws ec2 describe-internet-gateways \
            --region $region \
            --filters "Name=attachment.vpc-id,Values=$vpc_id" \
            --query 'InternetGateways[*].InternetGatewayId' \
            --output text)
        
        for igw_id in $igw_ids; do
            echo "Detaching and deleting Internet Gateway: $igw_id"
            execute_aws_command "aws ec2 detach-internet-gateway --region $region --internet-gateway-id $igw_id --vpc-id $vpc_id"
            execute_aws_command "aws ec2 delete-internet-gateway --region $region --internet-gateway-id $igw_id"
        done
        
        # 3. Delete Subnets
        subnet_ids=$(aws ec2 describe-subnets \
            --region $region \
            --filters "Name=vpc-id,Values=$vpc_id" \
            --query 'Subnets[*].SubnetId' \
            --output text)
        
        for subnet_id in $subnet_ids; do
            echo "Deleting Subnet: $subnet_id"
            execute_aws_command "aws ec2 delete-subnet --region $region --subnet-id $subnet_id"
        done
        
        # 4. Delete Route Tables (except main)
        rt_ids=$(aws ec2 describe-route-tables \
            --region $region \
            --filters "Name=vpc-id,Values=$vpc_id" \
            --query 'RouteTables[?Associations[0].Main != `true`].RouteTableId' \
            --output text)
        
        for rt_id in $rt_ids; do
            echo "Deleting Route Table: $rt_id"
            execute_aws_command "aws ec2 delete-route-table --region $region --route-table-id $rt_id"
        done
        
        # 5. Delete Security Groups (except default)
        sg_ids=$(aws ec2 describe-security-groups \
            --region $region \
            --filters "Name=vpc-id,Values=$vpc_id" \
            --query 'SecurityGroups[?GroupName != `default`].GroupId' \
            --output text)
        
        for sg_id in $sg_ids; do
            echo "Deleting Security Group: $sg_id"
            execute_aws_command "aws ec2 delete-security-group --region $region --group-id $sg_id"
        done
        
        # Finally, delete the VPC
        echo "Deleting VPC: $vpc_id"
        execute_aws_command "aws ec2 delete-vpc --region $region --vpc-id $vpc_id"
    done
done

echo "VPC deletion process completed"
