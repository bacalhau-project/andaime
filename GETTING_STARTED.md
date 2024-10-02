# Andaime

**Version:** v0.0.1-alpha

## Table of Contents

- [Andaime](#andaime)
  - [Table of Contents](#table-of-contents)
  - [Introduction](#introduction)
  - [Features](#features)
  - [Prerequisites](#prerequisites)
  - [Installation](#installation)
  - [Configuration](#configuration)
    - [Environment Variables](#environment-variables)
    - [Configuration File](#configuration-file)
    - [Command-Line Flags](#command-line-flags)
  - [Usage](#usage)
    - [Commands](#commands)
      - [Create](#create)
      - [Destroy](#destroy)
      - [List](#list)
      - [Version](#version)
  - [Examples](#examples)
    - [Creating AWS Resources](#creating-aws-resources)
    - [Destroying AWS Resources](#destroying-aws-resources)
    - [Listing AWS Resources](#listing-aws-resources)
    - [Checking Version](#checking-version)
  - [Project Structure](#project-structure)
  - [Contributing](#contributing)
  - [License](#license)

## Introduction

**Andaime** is a command-line tool designed to streamline the management of AWS resources for the Bacalhau deployment. Utilizing the power of the [Cobra](https://github.com/spf13/cobra) library, Andaime provides intuitive commands to create, destroy, and list AWS infrastructure components essential for deploying and maintaining Bacalhau nodes across multiple AWS regions.

## Features

- **Create AWS Resources:** Automate the provisioning of VPCs, Subnets, Security Groups, and EC2 Instances tailored for Bacalhau deployment.
- **Destroy AWS Resources:** Cleanly tear down all AWS resources associated with the Bacalhau project, ensuring no residual infrastructure remains.
- **List AWS Resources:** Generate comprehensive reports of all AWS resources tagged under the Bacalhau project across all regions.
- **Versioning:** Easily check the current version of Andaime.
- **Flexible Configuration:** Configure Andaime using environment variables, configuration files, or command-line flags.
- **Verbose Mode:** Enable detailed logging for troubleshooting and monitoring purposes.

## Prerequisites

Before getting started with Andaime, ensure you have the following prerequisites installed and configured:

1. **Go Programming Language:** Andaime is built using Go. Install Go from [here](https://golang.org/dl/).

2. **AWS Account:** You need an AWS account with appropriate permissions to create and manage resources.

3. **AWS CLI:** Install and configure the AWS CLI to manage your AWS credentials and settings.
   - **Installation:** [AWS CLI Installation Guide](https://docs.aws.amazon.com/cli/latest/userguide/cli-chap-install.html)
   - **Configuration:** Run `aws configure` and provide your AWS Access Key, Secret Key, Default Region, and Output Format.

4. **Git:** To clone the repository.
   - **Installation:** [Git Downloads](https://git-scm.com/downloads)

## Installation

Follow these steps to install and set up Andaime:

1. **Clone the Repository:**

   ```bash
   git clone https://github.com/yourusername/andaime.git
   cd andaime
   ```

2. **Build the Tool:**

   Ensure you have Go installed and set up in your `PATH`.

   ```bash
   go build -o andaime
   ```

   This command compiles the Andaime tool and generates an executable named `andaime` in the current directory.

3. **(Optional) Install Dependencies:**

   The project uses Go modules to manage dependencies. Ensure all dependencies are fetched:

   ```bash
   go mod download
   ```

## Configuration

Andaime can be configured using **Environment Variables**, a **Configuration File**, or **Command-Line Flags**. The precedence of configuration is as follows:

1. **Command-Line Flags** override
2. **Environment Variables**, which in turn override
3. **Configuration File** settings.

### Environment Variables

Set the following environment variables to configure Andaime:

- `PROJECT_NAME`: Name of your project. *(Default: "bacalhau-by-andaime")*
- `TARGET_PLATFORM`: Target platform for deployment. *(Default: "aws")*
- `NUMBER_OF_ORCHESTRATOR_NODES`: Number of orchestrator nodes. *(Default: 1)*
- `NUMBER_OF_COMPUTE_NODES`: Number of compute nodes. *(Default: 2)*

**Example:**

```bash
export PROJECT_NAME="my-bacalhau-project"
export TARGET_PLATFORM="aws"
export NUMBER_OF_ORCHESTRATOR_NODES=2
export NUMBER_OF_COMPUTE_NODES=4
```

### Configuration File

Andaime looks for a `config.json` file in the current directory. This file can specify the same settings as environment variables.

**Sample `config.json`:**

```json
{
  "PROJECT_NAME": "my-bacalhau-project",
  "TARGET_PLATFORM": "aws",
  "NUMBER_OF_ORCHESTRATOR_NODES": 2,
  "NUMBER_OF_COMPUTE_NODES": 4
}
```

**Notes:**

- If `config.json` does not exist, Andaime will skip reading it.
- Ensure the JSON structure matches the expected keys.

### Command-Line Flags

You can override configuration settings using command-line flags when running Andaime commands.

**Available Flags:**

- `--project-name`: Set project name.
- `--target-platform`: Set target platform.
- `--orchestrator-nodes`: Set number of orchestrator nodes.
- `--compute-nodes`: Set number of compute nodes.
- `--target-regions`: Comma-separated list of target AWS regions. *(Default: "us-east-1")*
- `--orchestrator-ip`: IP address of an existing orchestrator node.
- `--aws-profile`: AWS profile to use for credentials.
- `--instance-type`: Instance type for both compute and orchestrator nodes. *(Default: "t2.medium")*
- `--compute-instance-type`: Instance type for compute nodes. Overrides `--instance-type` for compute nodes.
- `--orchestrator-instance-type`: Instance type for orchestrator nodes. Overrides `--instance-type` for orchestrator nodes.
- `--volume-size`: Volume size of each node created in Gigabytes. *(Default: 8)*
- `--verbose`: Enable verbose output.

**Example:**

```bash
./andaime aws create --project-name "my-project" --orchestrator-nodes 2 --compute-nodes 4 --verbose
```

## Usage

Andaime provides several commands to manage AWS resources for your Bacalhau deployment. Below are the primary commands and their descriptions.

### Commands

#### Create

**Description:**  
Create AWS resources required for the Bacalhau deployment. This includes VPCs, Subnets, Security Groups, and EC2 Instances.

**Usage:**

```bash
./andaime aws create [flags]
```

**Flags:**

- `--project-name`: Set project name.
- `--target-platform`: Set target platform.
- `--orchestrator-nodes`: Set number of orchestrator nodes.
- `--compute-nodes`: Set number of compute nodes.
- `--target-regions`: Comma-separated list of target AWS regions.
- `--orchestrator-ip`: IP address of an existing orchestrator node.
- `--aws-profile`: AWS profile to use for credentials.
- `--instance-type`: Instance type for both compute and orchestrator nodes.
- `--compute-instance-type`: Instance type for compute nodes.
- `--orchestrator-instance-type`: Instance type for orchestrator nodes.
- `--volume-size`: Volume size of each node created (GB).
- `--verbose`: Enable verbose output.

**Example:**

```bash
./andaime aws create --project-name "my-project" --orchestrator-nodes 2 --compute-nodes 4 --target-regions "us-east-1,us-west-2" --verbose
```

#### Destroy

**Description:**  
Destroy all AWS resources associated with the Bacalhau deployment. This will remove VPCs, Subnets, Security Groups, and EC2 Instances tagged with `project: andaime`.

**Usage:**

```bash
./andaime aws destroy [flags]
```

**Flags:**  
Same as the `create` command, excluding some specific to creation.

**Example:**

```bash
./andaime aws destroy --verbose
```

#### List

**Description:**  
List all AWS resources tagged with `project: andaime` across all regions. Generates a comprehensive report of VPCs, Subnets, Security Groups, Internet Gateways, Route Tables, and Instances.

**Usage:**

```bash
./andaime aws list [flags]
```

**Flags:**  
Same as the `create` command.

**Example:**

```bash
./andaime aws list --verbose
```

#### Version

**Description:**  
Print the current version of Andaime.

**Usage:**

```bash
./andaime aws version
```

**Example:**

```bash
./andaime aws version
# Output: v0.0.1-alpha
```

## Examples

### Creating AWS Resources

Deploy a Bacalhau project with 2 orchestrator nodes and 4 compute nodes across `us-east-1` and `us-west-2` regions with verbose output:

```bash
./andaime aws create \
  --project-name "my-bacalhau-project" \
  --orchestrator-nodes 2 \
  --compute-nodes 4 \
  --target-regions "us-east-1,us-west-2" \
  --verbose
```

### Destroying AWS Resources

Remove all resources associated with the Bacalhau project with verbose output:

```bash
./andaime aws destroy --verbose
```

### Listing AWS Resources

Generate a report of all resources tagged with `project: andaime`:

```bash
./andaime aws list --verbose
```

### Checking Version

Display the current version of Andaime:

```bash
./andaime aws version
# Output: v0.0.1-alpha
```

## Project Structure

Here's an overview of the main components of the Andaime project:

```
andaime/
├── cmd/
│   └── aws.go          # Core CLI commands and functionality
├── startup_scripts/    # Embedded startup scripts for EC2 instances
├── config.json         # Optional configuration file
├── go.mod              # Go module file
├── go.sum              # Go checksum file
├── README.md           # Project documentation
└── ...
```

- **`cmd/aws.go`**: Contains the implementation of the CLI commands (`create`, `destroy`, `list`, `version`) using the Cobra library.
- **`startup_scripts/`**: Directory containing scripts that are executed upon EC2 instance initialization. These scripts are embedded into the binary using Go's `embed` package.
- **`config.json`**: Optional JSON file for configuring Andaime settings. Overrides default settings but can be overridden by environment variables and flags.
- **`go.mod` & `go.sum`**: Manage Go dependencies.

## Contributing

Contributions are welcome! To contribute to Andaime, follow these steps:

1. **Fork the Repository:**

   Click the "Fork" button at the top right of the repository page to create your own fork.

2. **Clone Your Fork:**

   ```bash
   USERNAME=yourusername
   git clone https://github.com/$USERNAME/andaime.git
   cd andaime
   ```

3. **Create a New Branch:**

   ```bash
   git checkout -b feature/$USERNAME-your-feature-name
   ```

4. **Make Your Changes:**

   Implement your feature or fix the bug.

5. **Commit Your Changes:**

   ```bash
   git commit -m "Add feature: your feature description"
   ```

6. **Push to Your Fork:**

   ```bash
   git push origin feature/$USERNAME-your-feature-name
   ```

7. **Open a Pull Request:**

   Navigate to the original repository and click on "Compare & pull request" to submit your changes for review.

**Please ensure your contributions adhere to the project's coding standards and include appropriate documentation and tests.**

## License

This project is licensed under the [MIT License](LICENSE).
