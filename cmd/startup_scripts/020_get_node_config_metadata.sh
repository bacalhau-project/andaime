#!/bin/bash

set -e

# Get vCPU count
VCPU_COUNT=$(nproc --all || echo "unknown")

# Get total memory in GB
MEMORY_TOTAL=$(free -m | awk '/^Mem:/{print $2}')
MEMORY_GB=$(awk "BEGIN {printf \"%.1f\", $MEMORY_TOTAL / 1024}")

# Get total disk size in GB
DISK_SIZE=$(df -BG / | awk 'NR==2 {print $2}' | sed 's/G//')

# Orchestrator IPs (assuming this is provided externally)
ORCHESTRATORS="{{.OrchestratorIPs}}"

# Write environment variables to /etc/node-config
cat << EOF > /etc/node-config
MACHINE_TYPE={{ .MachineType }}
NODE_TYPE={{.NodeType}}
VCPU_COUNT=$VCPU_COUNT
MEMORY_GB=$MEMORY_GB
DISK_GB=$DISK_SIZE
ORCHESTRATORS=$ORCHESTRATORS
HOSTNAME=$HOSTNAME
TOKEN=$TOKEN
PROJECT_NAME={{.ProjectName}}
LOCATION={{.Location}}
PUBLIC_IP={{.PublicIP}}
EOF

chmod 644 /etc/node-config

echo "Node configuration has been written to /etc/node-config"
cat /etc/node-config