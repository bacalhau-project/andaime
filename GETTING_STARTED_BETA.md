# Getting Started with Andaime Beta Features

Copyright (c) 2024 Expanso oss@expanso.io
This software is licensed under the Apache2/MIT License.

This was generated by Claude Sonnet 3.5, with the assistance of my human mentor.

A guide to using Andaime's beta features for Azure and GCP deployments. Cloud-hopping has never been so easy!

## Table of Contents

- [Getting Started with Andaime Beta Features](#getting-started-with-andaime-beta-features)
  - [Table of Contents](#table-of-contents)
  - [Introduction](#introduction)
  - [Prerequisites](#prerequisites)
  - [Installation](#installation)
  - [Azure Deployment](#azure-deployment)
    - [Azure Configuration](#azure-configuration)
    - [Configuration File Structure](#configuration-file-structure)
    - [Azure Usage](#azure-usage)
  - [GCP Deployment](#gcp-deployment)
    - [GCP Configuration](#gcp-configuration)
    - [Configuration File Structure](#configuration-file-structure-1)
    - [GCP Usage](#gcp-usage)

## Introduction

Andaime is expanding its capabilities beyond AWS to include Azure and GCP deployments. This guide will walk you through the process of using these beta features to create and manage Bacalhau clusters on these cloud platforms.

## Prerequisites

- Go 1.16 or later
- Azure CLI (for Azure deployments)
- Google Cloud SDK (for GCP deployments)
- Andaime CLI tool (built from the latest source)

## Installation

1. Clone the Andaime repository:
   ```bash
   git clone https://github.com/bacalhau-project/andaime.git
   cd andaime
   ```

2. Build the project:
   ```bash
   make build
   ```

## Azure Deployment

### Azure Configuration

Before using the Azure deployment feature, ensure you have:

1. An Azure account and subscription
2. Azure CLI installed and authenticated
3. The necessary permissions to create resources in your Azure subscription
4. A configuration file (e.g., `config-azure.yaml`) with the required settings

### Configuration File Structure

The Azure deployment is driven by a configuration file. Here's an explanation of the available settings:

```yaml
azure:
  allowed_ports:
    - 22    # SSH
    - 1234  # Custom port
    - 1235  # Custom port
    - 4222  # NATS
  default_count_per_zone: 1
  default_disk_size_gb: 30
  default_machine_type: Standard_D2s_v3
  machines:
    - location: eastus2
      parameters:
        count: 2
    - location: australiaeast
      parameters:
        type: Standard_B4as_v2
    - location: spaincentral
      parameters:
        count: 2
        orchestrator: false
        type: Standard_D2s_v3
    - location: uksouth
      parameters:
        orchestrator: true
    - location: brazilsouth
      parameters:
        count: 4
        vmsize: Standard_B4as_v2
    - location: westus
  resource_group_location: eastus
  subscription_id: fd075559-43fc-4fbe-8dee-21e125a9dafc

general:
  bacalhau_settings:
    - key: compute.allowlistedlocalpaths
      value:
        - /tmp
        - /data
    - key: compute.heartbeat.interval
      value: 5s
    - key: compute.heartbeat.infoupdateinterval
      value: 5s
    - key: compute.heartbeat.resourceupdateinterval
      value: 5s
    - key: orchestrator.nodemanager.disconnecttimeout
      value: 5s
    - key: user.installationid
      value: BACA14A0-eeee-eeee-eeee-194519911992
    - key: jobadmissioncontrol.acceptnetworkedjobs
      value: true
  custom_script_path: /Users/daaronch/code/andaime/scripts/install_k3s.sh
  log_level: debug
  log_path: /var/log/andaime
  project_prefix: andaime
  ssh_port: 22
  ssh_private_key_path: /Users/daaronch/.ssh/id_ed25519
  ssh_public_key_path: /Users/daaronch/.ssh/id_ed25519.pub
  ssh_user: andaime
```

Explanation of settings:

1. `azure` section:
   - `allowed_ports`: List of ports to open in the security group
   - `default_count_per_zone`: Default number of VMs to create per zone if not specified
   - `default_disk_size_gb`: Default disk size for VMs in GB
   - `default_machine_type`: Default VM size if not specified
   - `machines`: List of machine configurations per location
     - `location`: Azure region for deployment
     - `parameters`:
       - `count`: Number of VMs to create in this location
       - `type` or `vmsize`: VM size for this location
       - `orchestrator`: Boolean to indicate if this is an orchestrator node
   - `resource_group_location`: Location for the Azure resource group
   - `subscription_id`: Your Azure subscription ID

2. `general` section:
   - `bacalhau_settings`: List of key-value pairs for Bacalhau configuration
   - `custom_script_path`: Path to a custom script to run on VM startup
   - `log_level`: Logging level for Andaime
   - `log_path`: Path for Andaime logs
   - `project_prefix`: Prefix for resource names
   - `ssh_port`: SSH port for connecting to VMs
   - `ssh_private_key_path`: Path to your SSH private key
   - `ssh_public_key_path`: Path to your SSH public key
   - `ssh_user`: SSH username for connecting to VMs

### Azure Usage

To create an Azure deployment using the configuration file:

1. Create a configuration file (e.g., `config-azure.yaml`) with your desired settings.
2. Run the following command:

```bash
./andaime beta azure create-deployment --config config-azure.yaml
```

This command will:
1. Read the configuration file
2. Create a new resource group (if it doesn't exist)
3. Set up a virtual network and subnet
4. Create public IP addresses and a network security group
5. Deploy the specified number of VMs for compute and orchestrator nodes in the specified locations
6. Install and configure Bacalhau on the nodes

## GCP Deployment

### GCP Configuration

Before using the GCP deployment feature, ensure you have:

1. A Google Cloud Platform account and project
2. Google Cloud SDK installed and authenticated
3. The necessary permissions to create resources in your GCP project
4. A configuration file (e.g., `config-gcp.yaml`) with the required settings

### Configuration File Structure

The GCP deployment is driven by a configuration file, similar to the Azure deployment. Here's an explanation of the available settings for GCP:

```yaml
gcp:
  project_id: your-gcp-project-id
  region: us-central1
  zone: us-central1-a
  allowed_ports:
    - 22    # SSH
    - 1234  # Custom port
    - 1235  # Custom port
    - 4222  # NATS
  default_count_per_zone: 1
  default_disk_size_gb: 30
  default_machine_type: e2-medium
  machines:
    - zone: us-central1-a
      parameters:
        count: 2
    - zone: us-west1-b
      parameters:
        type: n1-standard-2
    - zone: us-east1-b
      parameters:
        count: 2
        orchestrator: false
        type: e2-standard-4
    - zone: europe-west1-b
      parameters:
        orchestrator: true
    - zone: asia-east1-a
      parameters:
        count: 4
        machine_type: n1-standard-4
  network_name: andaime-network
  subnetwork_name: andaime-subnet

general:
  bacalhau_settings:
    - key: node.allowlistedlocalpaths
      value:
        - /tmp
        - /data
    - key: node.compute.controlplanesettings.heartbeatfrequency
      value: 5s
    - key: node.requester.controlplanesettings.heartbeatcheckfrequency
      value: 5s
    - key: node.requester.controlplanesettings.nodedisconnectedafter
      value: 5s
    - key: user.installationid
      value: BACA14A0-eeee-eeee-eeee-194519911992
    - key: node.requester.jobselectionpolicy.acceptnetworkedjobs
      value: true
    - key: node.compute.jobselection.acceptnetworkedjobs
      value: true
  custom_script_path: /path/to/your/custom/script.sh
  log_level: debug
  log_path: /var/log/andaime
  project_prefix: andaime
  ssh_port: 22
  ssh_private_key_path: /path/to/.ssh/id_rsa
  ssh_public_key_path: /path/to/.ssh/id_rsa.pub
  ssh_user: andaime
```

Explanation of settings:

1. `gcp` section:
   - `project_id`: Your GCP project ID
   - `region`: Default GCP region for deployment
   - `zone`: Default GCP zone for deployment
   - `allowed_ports`: List of ports to open in the firewall rules
   - `default_count_per_zone`: Default number of VMs to create per zone if not specified
   - `default_disk_size_gb`: Default disk size for VMs in GB
   - `default_machine_type`: Default machine type if not specified
   - `machines`: List of machine configurations per zone
     - `zone`: GCP zone for deployment
     - `parameters`:
       - `count`: Number of VMs to create in this zone
       - `type` or `machine_type`: Machine type for this zone
       - `orchestrator`: Boolean to indicate if this is an orchestrator node
   - `network_name`: Name of the VPC network to create or use
   - `subnetwork_name`: Name of the subnetwork to create or use

2. `general` section:
   - `bacalhau_settings`: List of key-value pairs for Bacalhau configuration
   - `custom_script_path`: Path to a custom script to run on VM startup
   - `log_level`: Logging level for Andaime
   - `log_path`: Path for Andaime logs
   - `project_prefix`: Prefix for resource names
   - `ssh_port`: SSH port for connecting to VMs
   - `ssh_private_key_path`: Path to your SSH private key
   - `ssh_public_key_path`: Path to your SSH public key
   - `ssh_user`: SSH username for connecting to VMs

### GCP Usage

To create a GCP deployment using the configuration file:

1. Create a configuration file (e.g., `config-gcp.yaml`) with your desired settings.
2. Run the following command:

```bash
./andaime beta gcp create-deployment --config config-gcp.yaml
```

This command will:
1. Read the configuration file
2. Create or use the specified VPC network and subnetwork
3. Set up firewall rules
4. Deploy the specified number of VMs for compute and orchestrator nodes in the specified zones
5. Install and configure Bacalhau on the nodes

This GETTING_STARTED_BETA.md file now provides a comprehensive guide for users to understand and use the beta features for both Azure and GCP deployments in Andaime, with both using configuration files instead of command-line flags.