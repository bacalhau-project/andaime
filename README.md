
# Andaime
_A CLI tool for generating an MVP for running a private Bacalhau cluster._

Andaime (Poruguese for "scaffolding", pronounced An-Dye-Me) is a command-line tool for managing AWS resources that can run a Bacalhau Network. It allows you to create, list, and destroy AWS infrastructure components, including VPCs, subnets, internet gateways, route tables, security groups, and EC2 instances.

## Prerequisites

- Go 1.16 or later
- AWS CLI configured with appropriate credentials
- AWS SDK for Go

## Installation and Building

Clone the repository:

```bash
git clone https://github.com/bacalhau-project/andaime.git
cd andaime
```

You can build the project with the Go compiler:
```bash
go build ./...
```

or, using the `makefile`

```bash
make build
```

If you wish to build for all supported platforms (Linux and macOS on `arm64` and `amd64` arch), you can run the following:

```bash
make release
```

This will build and tarball Andaime for all of the aforementioned targets in a `./releases` directory with the filenames of `andaime-${OS}-${ARCHITECTURE}.tar.gz`.

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
AWS_ACCESS_KEY_ID
AWS_SECRET_ACCESS_KEY
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

# Create a single node of a specific type, and add them to an orchestrator using environment variables to authenticate with AWS
AWS_ACCESS_KEY_ID=<YOUR_KEY_ID> AWS_SECRET_ACCESS_KEY=<YOUR_ACCESS_KEY> ./andaime --target-regions="us-east-1" --compute-nodes=1 --instance-type="t2.large" --orchestrator-ip=<ORCHESTRATOR_IP_ADDRESS>

```

#### List Resources
```bash
./andaime list
```
#### Destroy Resources
```bash
./andaime destroy
```