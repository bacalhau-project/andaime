#!/bin/bash

# Install K3s with minimal resources
curl -sfL https://get.k3s.io | INSTALL_K3S_EXEC="--disable traefik --disable servicelb --disable-cloud-controller --disable-network-policy --kubelet-arg=\"eviction-hard=memory.available<100Mi,nodefs.available<1Gi,imagefs.available<1Gi\" --kube-controller-manager-arg=\"node-monitor-period=60s\" --kube-controller-manager-arg=\"node-monitor-grace-period=120s\"" sh -

# Wait for K3s to be ready
until kubectl get nodes | grep -q " Ready"; do
    echo "Waiting for K3s to be ready..."
    sleep 5
done

echo "K3s installed and ready!"

# Optional: Display node information
kubectl get nodes -o wide
