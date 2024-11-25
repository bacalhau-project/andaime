#!/usr/bin/env bash

BACALHAU_INSTALL_ID=${BACALHAU_INSTALL_ID:-"BACA14A0-4EEE-EEEE-8EEE-EEEEEEEEEEEE"}

ensure_jq_installed() {
    if ! command -v jq &> /dev/null; then
        log "jq is not installed. Attempting to install..."
        if command -v apt-get &> /dev/null; then
            sudo apt-get update && sudo apt-get install -y jq
        elif command -v yum &> /dev/null; then
            sudo yum install -y jq
        elif command -v dnf &> /dev/null; then
            sudo dnf install -y jq
        elif command -v brew &> /dev/null; then
            brew install jq
        else
            log "Error: Unable to install jq. Please install it manually."
            exit 1
        fi
        log "jq has been installed successfully."
    else
        log "jq is already installed."
    fi
}

ensure_jq_installed

# Replace the last 12 characters of the BACALHAU_INSTALL_ID with random 12 characters
BACALHAU_INSTALL_ID=$(echo "${BACALHAU_INSTALL_ID}" | \
    python3 -c "
import sys
import uuid

id = sys.stdin.read().strip()
new_id = id[:-12] + uuid.uuid4().hex[:12]
print(new_id)
    ")

sudo curl -sSL "https://get.bacalhau.org/install.sh?dl=${BACALHAU_INSTALL_ID}" | sudo bash

