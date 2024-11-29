
# Andaime
_A CLI tool for generating an MVP for running a private Bacalhau cluster._

Andaime (Poruguese for "scaffolding", pronounced An-Dye-Me) is a command-line tool for managing AWS resources that can run a Bacalhau Network. It allows you to create, list, and destroy AWS infrastructure components, including VPCs, subnets, internet gateways, route tables, security groups, and EC2 instances.

## Prerequisites

- Go 1.21 or later
- Cloud Provider CLI (AWS, GCP) configured with appropriate credentials
- Just task runner (recommended)

## Installation and Building

### Prerequisites Installation

1. Install Go (1.21+):
```bash
# macOS (using Homebrew)
brew install go

# Linux
# Follow official Go installation guide: https://golang.org/doc/install
```

2. Install Just task runner:
```bash
# macOS
brew install just

# Linux
# https://github.com/casey/just#installation
```

### Repository Setup

Clone the repository:
```bash
git clone https://github.com/bacalhau-project/andaime.git
cd andaime
```

### Building the Project

You have multiple build options:

#### Quick Build
```bash
# Using Go directly
go build ./cmd/andaime

# Using Just (recommended)
just build
```

#### Multi-Platform Release Build
```bash
# Build release artifacts for Linux, macOS (Intel and Apple Silicon)
just build-release
```

This will create platform-specific binaries in the `dist/` directory:
- `andaime_linux_amd64`
- `andaime_darwin_amd64`
- `andaime_darwin_arm64`
- `andaime_windows_amd64.exe`

### Development Tools

For development, install additional tools:
```bash
# Generate mocks
just genmock

# Generate cloud data
just gencloud
```

## Usage
Commands
```
./andaime <ACTION> <OPTIONS>

create: Create nodes and supporting infrastructure
destroy: Destroy nodes and supporting infrastructure
list: List resources tagged with project: andaime

Options
--project-name: Set project name
--target-platform: Set target platform (default: aws)
--orchestrator-nodes: Set number of orchestrator nodes (default: 1, Max: 1)
--compute-nodes: Set number of compute nodes (default: 2)
--orchestrator-ip: IP address of existing orchestrator node. Overrides --orchestrator-nodes flag
--aws-profile: AWS profile to use for credentials (default: default)
--target-regions: Comma-separated list of target AWS regions (default: us-east-1). Created in a round-robin order.
--instance-type: The instance type for both the compute and orchestrator nodes
--compute-instance-type: The instance type for the compute nodes. Overrides --instance-type for compute nodes.
--orchestrator-instance-type: The instance type for the orchestrator nodes. Overrides --instance-type for orchestrator nodes.
--volume-size: The volume size of each node created (Gigabytes). Default: 8

--help: Show help message.
--verbose: Enable verbose logging.
```

## Configuration and Customization

Andaime supports multiple configuration methods to suit different workflows:

### 1. Configuration File (config.json)
Create a `config.json` in the project root:

```json
{
  "PROJECT_NAME": "bacalhau-cluster",
  "TARGET_PLATFORM": ["aws", "gcp"],
  "ORCHESTRATOR_NODES": {
    "count": 1,
    "instance_type": "t3.medium"
  },
  "COMPUTE_NODES": {
    "count": 3,
    "instance_type": "c5.large"
  },
  "REGIONS": ["us-east-1", "us-west-2", "eu-west-1"]
}
```

### 2. Environment Variables
Override or supplement configuration via environment variables:

```bash
# Cloud Provider Credentials
export AWS_ACCESS_KEY_ID=your_aws_key
export AWS_SECRET_ACCESS_KEY=your_aws_secret
export GCP_PROJECT_ID=your_gcp_project
export ANDAIME_AWS_KEY_PAIR_NAME=andaime-local-key

# Cluster Configuration
export ANDAIME_PROJECT_NAME="my-bacalhau-cluster"
export ANDAIME_TARGET_PLATFORM="aws"
export ANDAIME_ORCHESTRATOR_NODES=1
export ANDAIME_COMPUTE_NODES=3
```

### 3. CLI Flags
Direct CLI configuration for maximum flexibility:

```bash
# Specify configuration directly
andaime create \
  --project-name "bacalhau-cluster" \
  --target-platform aws \
  --orchestrator-nodes 1 \
  --compute-nodes 3 \
  --instance-type t3.medium \
  --target-regions us-east-1,us-west-2 \
  --aws-key-pair-name andaime-local-key
```

### Configuration Precedence
Andaime resolves configuration in this order:
1. CLI Flags (Highest Priority)
2. Environment Variables
3. Configuration File
4. Default Values

### Supported Cloud Providers
- AWS (Primary)
- GCP (Beta)
- More providers planned

## Workflow Examples

### 1. Basic AWS Cluster Creation
```bash
# Create a simple Bacalhau cluster in us-east-1
andaime create \
  --target-platform aws \
  --target-regions us-east-1 \
  --compute-nodes 3 \
  --orchestrator-nodes 1
```

### 2. Multi-Region GCP Deployment
```bash
# Deploy a cluster across multiple GCP regions
andaime create \
  --target-platform gcp \
  --target-regions us-central1,us-west1,europe-west3 \
  --compute-nodes 5 \
  --orchestrator-nodes 2 \
  --instance-type n2-standard-4
```

### 3. Existing Orchestrator Expansion
```bash
# Add compute nodes to an existing orchestrator
andaime create \
  --orchestrator-ip 203.0.113.10 \
  --compute-nodes 10 \
  --target-regions us-east-1,us-west-2
```

### 4. Resource Management
```bash
# List current Andaime-managed resources
andaime list

# Destroy all resources for the current project
andaime destroy
```

### 5. Advanced Configuration
```bash
# Use a specific AWS profile and custom configuration
AWS_PROFILE=my-bacalhau-profile \
andaime create \
  --config-file custom_config.json \
  --verbose
```

## Troubleshooting

- Ensure cloud provider credentials are correctly configured
- Check network connectivity and firewall rules
- Use `--verbose` flag for detailed logging
- Consult documentation for provider-specific requirements
