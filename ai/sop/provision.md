# Standard Operating Procedure: Node Provisioning

This document outlines the standard operating procedure for provisioning new nodes in the Bacalhau network, whether they are orchestrator or compute nodes.

## Prerequisites

Before beginning the provisioning process, ensure you have:

1. SSH access to the target machine
2. Valid private key in PEM format
3. Sudo privileges on the target machine
4. For compute nodes: A running orchestrator node's IP address

## System Requirements

Target machine must meet these minimum requirements:

- Operating System: Ubuntu 20.04 LTS or newer
- CPU: 2+ cores
- RAM: 4GB minimum (8GB recommended)
- Storage: 20GB available disk space
- Network: Public IP address and unrestricted outbound internet access

## Provisioning Steps

### 1. Initial Connection Verification
- Verify SSH connectivity
- Check sudo access
- Validate system requirements
- Record initial system state

### 2. System Preparation
- Update package lists
- Upgrade existing packages
- Install required dependencies:
  - curl
  - wget
  - git
  - build-essential

### 3. Docker Installation
- Remove existing Docker installations
- Install Docker repository
- Install Docker CE
- Add user to Docker group
- Enable Docker service
- Verify Docker installation

### 4. Bacalhau Installation
- Download Bacalhau installer
- Run installation script
- Verify Bacalhau binary installation
- Set up Bacalhau service directory

### 5. Node Configuration
For Orchestrator Nodes:
- Configure as orchestrator
- Set up network ports
- Initialize node keys

For Compute Nodes:
- Configure as compute node
- Connect to orchestrator
- Set resource limits

### 6. Service Configuration
- Create systemd service file
- Enable automatic startup
- Start Bacalhau service

### 7. Verification
- Check service status
- Verify node registration
- Run test job (for compute nodes)
- Monitor logs for errors

## Troubleshooting

Common issues and their solutions:

1. SSH Connection Issues
   - Check network connectivity
   - Verify key permissions
   - Confirm username and IP

2. Docker Installation Failures
   - Check system requirements
   - Verify package repository access
   - Review Docker logs

3. Bacalhau Service Issues
   - Check service logs
   - Verify configuration
   - Confirm network connectivity

## Rollback Procedure

If provisioning fails:

1. Stop Bacalhau service
2. Remove Bacalhau installation
3. Remove Docker (if requested)
4. Document failure reason
5. Clean up system changes

## Success Criteria

A successful provisioning must meet these criteria:

1. All services running without errors
2. Node properly registered in network
3. Test job execution successful (for compute nodes)
4. All system services stable
5. Monitoring and logging functional

## Documentation

After successful provisioning, document:

1. Node IP address and type
2. Installation date and version
3. Any custom configurations
4. Contact information for node operator
5. Monitoring endpoints
