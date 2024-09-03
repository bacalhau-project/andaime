#!/bin/bash

SSH_USER="{{.SSHUser}}"
PUBLIC_KEY_MATERIAL_B64="{{.PublicKeyMaterialB64}}"
PUBLIC_KEY_MATERIAL=$(echo "${PUBLIC_KEY_MATERIAL_B64}" | sudo base64 -d)

echo "Startup script started..." >> /tmp/startup_log.txt
sudo useradd -m "${SSH_USER}"
sudo usermod -aG sudo "${SSH_USER}"
echo "${SSH_USER} ALL=(ALL) NOPASSWD:ALL" | sudo EDITOR='tee -a' visudo

# Copy the public key to the authorized_keys file for that user
sudo mkdir -p /home/${SSH_USER}/.ssh
echo "${PUBLIC_KEY_MATERIAL}" >> /home/${SSH_USER}/.ssh/authorized_keys
sudo chmod 600 /home/${SSH_USER}/.ssh/authorized_keys
sudo chown ${SSH_USER}:${SSH_USER} /home/${SSH_USER}/.ssh/authorized_keys

echo "Startup script completed." >> /tmp/startup_log.txt