
# Andaime
_A CLI tool for generating an MVP for running a private Bacalhau cluster._

Andaime (Poruguese for "scaffolding") is a command-line tool for managing AWS resources for the Bacalhau project. It allows you to create, list, and destroy AWS infrastructure components, including VPCs, subnets, internet gateways, route tables, security groups, and EC2 instances.

## Prerequisites

- Go 1.16 or later
- AWS CLI configured with appropriate credentials
- AWS SDK for Go

## Installation and Building

Clone the repository:

```bash
git clone https://github.com/yourusername/andaime.git
cd andaime
```

Build the project:
```bash
go build ./...
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

--help: Show help message

```

## Configuration
You can configure the script using a config.json file in the root directory. The following parameters can be set:

```
PROJECT_NAME
TARGET_PLATFORM
NUMBER_OF_ORCHESTRATOR_NODES
NUMBER_OF_COMPUTE_NODES

```

Example config.json:
```json
{
  "PROJECT_NAME": "bacalhau-by-andaime",
  "TARGET_PLATFORM": "aws",
  "NUMBER_OF_ORCHESTRATOR_NODES": 1,
  "NUMBER_OF_COMPUTE_NODES": 2
}
```
## Environment Variables
You can also configure the script using environment variables:

```
PROJECT_NAME
TARGET_PLATFORM
NUMBER_OF_ORCHESTRATOR_NODES
NUMBER_OF_COMPUTE_NODES
```
Example:

```bash
export PROJECT_NAME="bacalhau-by-andaime"
export TARGET_PLATFORM="aws"
export NUMBER_OF_ORCHESTRATOR_NODES=1
export NUMBER_OF_COMPUTE_NODES=2
```

## Examples

#### Create Resources
```bash
# Create a new network
./andaime create --target-regions "us-east-1,us-west-2" --compute-nodes 3 --orchestrator-nodes=1

# Create nodes and add them to an existing orchestrator
./andaime create --aws-profile="expanso" --target-regions="us-east-1,us-west-2,eu-west-1,eu-west-2,eu-west-3,ap-southeast-1,ap-southeast-2,sa-east-1,ca-central-1,eu-north-1" --compute-nodes=20 --orchestrator-ip=<ORCHESTRATOR_IP_ADDRESS>


```

#### List Resources
```bash
./andaime list
```
#### Destroy Resources
```bash
./andaime destroy
```