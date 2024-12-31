#!/bin/bash
set -euo pipefail

# Get all regions
REGIONS=$(aws ec2 describe-regions --query "Regions[].RegionName" --output text)

# Function to list resources
list_resources() {
    local region=$1
    echo "=== Resources in $region ==="
    
    # List VPCs
    echo "VPCs:"
    aws ec2 describe-vpcs --region $region --filters Name=tag:project,Values=andaime \
        --query "Vpcs[].VpcId" --output text | xargs -n1 echo "  -"
    
    # List Subnets
    echo "Subnets:"
    aws ec2 describe-subnets --region $region --filters Name=tag:project,Values=andaime \
        --query "Subnets[].SubnetId" --output text | xargs -n1 echo "  -"
    
    # List Internet Gateways
    echo "Internet Gateways:"
    aws ec2 describe-internet-gateways --region $region --filters Name=tag:project,Values=andaime \
        --query "InternetGateways[].InternetGatewayId" --output text | xargs -n1 echo "  -"
    
    # List Route Tables
    echo "Route Tables:"
    aws ec2 describe-route-tables --region $region --filters Name=tag:project,Values=andaime \
        --query "RouteTables[].RouteTableId" --output text | xargs -n1 echo "  -"
    
    # List Security Groups
    echo "Security Groups:"
    aws ec2 describe-security-groups --region $region --filters Name=tag:project,Values=andaime \
        --query "SecurityGroups[].GroupId" --output text | xargs -n1 echo "  -"
    
    # List Instances
    echo "Instances:"
    aws ec2 describe-instances --region $region --filters Name=tag:project,Values=andaime \
        --query "Reservations[].Instances[].InstanceId" --output text | xargs -n1 echo "  -"
    
    echo ""
}

# Function to delete resources
delete_resources() {
    local region=$1
    
    # Terminate instances
    INSTANCES=$(aws ec2 describe-instances --region $region --filters Name=tag:project,Values=andaime \
        --query "Reservations[].Instances[].InstanceId" --output text)
    if [ -n "$INSTANCES" ]; then
        echo "Terminating instances in $region..."
        aws ec2 terminate-instances --region $region --instance-ids $INSTANCES
        aws ec2 wait instance-terminated --region $region --instance-ids $INSTANCES
    fi
    
    # Delete security groups
    SGS=$(aws ec2 describe-security-groups --region $region --filters Name=tag:project,Values=andaime \
        --query "SecurityGroups[?GroupName!='default'].GroupId" --output text)
    if [ -n "$SGS" ]; then
        echo "Deleting security groups in $region..."
        for SG in $SGS; do
            aws ec2 delete-security-group --region $region --group-id $SG
        done
    fi
    
    # Delete subnets
    SUBNETS=$(aws ec2 describe-subnets --region $region --filters Name=tag:project,Values=andaime \
        --query "Subnets[].SubnetId" --output text)
    if [ -n "$SUBNETS" ]; then
        echo "Deleting subnets in $region..."
        for SUBNET in $SUBNETS; do
            aws ec2 delete-subnet --region $region --subnet-id $SUBNET
        done
    fi
    
    # Detach and delete internet gateways
    IGWS=$(aws ec2 describe-internet-gateways --region $region --filters Name=tag:project,Values=andaime \
        --query "InternetGateways[].InternetGatewayId" --output text)
    if [ -n "$IGWS" ]; then
        echo "Deleting internet gateways in $region..."
        for IGW in $IGWS; do
            VPC=$(aws ec2 describe-internet-gateways --region $region --internet-gateway-ids $IGW \
                --query "InternetGateways[].Attachments[].VpcId" --output text)
            if [ -n "$VPC" ]; then
                aws ec2 detach-internet-gateway --region $region --internet-gateway-id $IGW --vpc-id $VPC
            fi
            aws ec2 delete-internet-gateway --region $region --internet-gateway-id $IGW
        done
    fi
    
    # Delete route tables
    RTS=$(aws ec2 describe-route-tables --region $region --filters Name=tag:project,Values=andaime \
        --query "RouteTables[?Associations[?Main!=true]].RouteTableId" --output text)
    if [ -n "$RTS" ]; then
        echo "Deleting route tables in $region..."
        for RT in $RTS; do
            aws ec2 delete-route-table --region $region --route-table-id $RT
        done
    fi
    
    # Delete VPCs
    VPCS=$(aws ec2 describe-vpcs --region $region --filters Name=tag:project,Values=andaime \
        --query "Vpcs[].VpcId" --output text)
    if [ -n "$VPCS" ]; then
        echo "Deleting VPCs in $region..."
        for VPC in $VPCS; do
            aws ec2 delete-vpc --region $region --vpc-id $VPC
        done
    fi
}

# Main script
echo "=== Listing all resources ==="
for REGION in $REGIONS; do
    list_resources $REGION
done

read -p "Are you sure you want to delete all these resources? (y/N) " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    for REGION in $REGIONS; do
        delete_resources $REGION
    done
    echo "Cleanup complete!"
else
    echo "Aborted."
fi
