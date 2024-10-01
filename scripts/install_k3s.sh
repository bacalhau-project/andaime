#!/bin/bash

# Install K3s (latest version) with minimal resource footprint
curl -sfL https://get.k3s.io | sh -s -

# Wait for K3s to be ready (with timeout)
echo "Waiting for K3s to be ready..."
until kubectl get nodes --request-timeout=5s 2>/dev/null | grep -q " Ready"; do
    sleep 5
done

echo "K3s installed and ready!"