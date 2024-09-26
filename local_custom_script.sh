#!/bin/bash

# Exit immediately if a command exits with a non-zero status
set -e

echo "Starting K3s installation script"

# Update package list and upgrade existing packages
echo "Updating and upgrading packages"
sudo apt-get update && sudo apt-get upgrade -y

# Install curl if not already installed
echo "Installing curl"
sudo apt-get install -y curl

# Download and install K3s
echo "Downloading and installing K3s"
curl -sfL https://get.k3s.io | sh -

# Wait for K3s to start (increase timeout to 2 minutes)
echo "Waiting for K3s to start"
for i in {1..24}; do
    if sudo systemctl is-active --quiet k3s; then
        echo "K3s is running"
        break
    fi
    if [ $i -eq 24 ]; then
        echo "K3s failed to start within 2 minutes"
        # Print K3s service status
        echo "K3s service status:"
        sudo systemctl status k3s
        # Check K3s logs
        echo "K3s logs:"
        sudo journalctl -u k3s
        exit 1
    fi
    echo "Waiting for K3s to start... (attempt $i)"
    sleep 5
done

# Get node information
echo "Node information:"
sudo kubectl get nodes

# Print K3s version
echo "K3s version:"
sudo kubectl version --short

# Print system resource usage
echo "System resource usage:"
free -m
df -h
top -bn1 | head -n 5

# Check for any error messages in the K3s log
echo "Last 20 lines of K3s log:"
sudo journalctl -u k3s -n 20

echo "K3s installation completed"