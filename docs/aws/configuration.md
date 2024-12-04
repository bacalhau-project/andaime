# AWS Provider Configuration Guide

## Configuration Options

### Required Settings
```yaml
aws:
  account_id: "123456789012"  # Your AWS account ID
  region: "us-west-2"         # AWS region for deployment
```

### Optional Settings
```yaml
aws:
  default_machine_type: "t3.micro"     # Default EC2 instance type
  default_disk_size_gb: 30             # Default EBS volume size
  default_count_per_zone: 1            # Instances per availability zone
  tags:                                # Custom resource tags
    Environment: "production"
    Project: "andaime"
```

### Machine Configuration
```yaml
aws:
  machines:
    - location: "us-west-2"
      parameters:
        count: 2
        machine_type: "t3.micro"
        orchestrator: true
```

## Network Configuration

### VPC Settings
- CIDR Block: 10.0.0.0/16
- Public Subnet: 10.0.1.0/24
- Private Subnet: 10.0.2.0/24

### Security
- Default security group with SSH access
- Internet gateway for public subnet
- NAT gateway for private subnet (optional)

## Resource Limits

### Default Quotas
- EC2 instances per region: 20
- VPCs per region: 5
- EBS volumes per instance: 40

### Performance
- Resource creation timeout: 30 minutes
- Polling interval: 10 seconds
- Maximum concurrent operations: 10

## Environment Variables
```bash
AWS_ACCESS_KEY_ID=your_access_key
AWS_SECRET_ACCESS_KEY=your_secret_key
AWS_DEFAULT_REGION=us-west-2
```

## Example Configuration
```yaml
aws:
  account_id: "123456789012"
  region: "us-west-2"
  default_machine_type: "t3.micro"
  default_disk_size_gb: 30
  machines:
    - location: "us-west-2"
      parameters:
        count: 2
        machine_type: "t3.large"
        orchestrator: true
    - location: "us-east-1"
      parameters:
        count: 3
        machine_type: "t3.medium"
  tags:
    Environment: "production"
    Project: "andaime"
    Team: "infrastructure"
```
