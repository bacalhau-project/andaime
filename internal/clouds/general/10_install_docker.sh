#!/usr/bin/env bash

# Exit immediately if a command exits with a non-zero status
set -e

# Variables
PRIMARY_MIRROR="http://archive.ubuntu.com/ubuntu/"
MIRRORS=(
    "http://azure.archive.ubuntu.com/ubuntu/"
    "http://us.archive.ubuntu.com/ubuntu/"
    "http://mirror.kernel.org/ubuntu/"
    "http://mirror.cs.uchicago.edu/ubuntu/"
    "http://mirror.math.princeton.edu/pub/ubuntu/"
)
#shellcheck disable=SC2034
BACKUP_MIRRORS=("${MIRRORS[@]}")
CURRENT_MIRROR="$PRIMARY_MIRROR"

RETRY_DELAY=5       # Seconds to wait between retries
MAX_RETRIES=3       # Maximum number of retries for critical commands

# Function to handle errors
error_exit() {
    echo "Error: $1" >&2
    exit 1
}

# Function to perform a command with retries
run_with_retries() {
    local -r -i max_attempts="$2"
    local -r cmd=("${@:3}")
    local attempt_num=1
    
    while (( attempt_num <= max_attempts )); do
        if "${cmd[@]}"; then
            return 0
        else
            echo "Attempt $attempt_num of $max_attempts failed for: ${cmd[*]}"
            if (( attempt_num == max_attempts )); then
                return 1
            else
                echo "Retrying in $RETRY_DELAY seconds..."
                sleep "$RETRY_DELAY"
            fi
        fi
        ((attempt_num++))
    done
}

# Function to update sources.list with a specified mirror
update_sources_list() {
    local mirror_url="$1"
    echo "Updating /etc/apt/sources.list to use mirror: $mirror_url"
    
    sudo cp /etc/apt/sources.list /etc/apt/sources.list.backup || error_exit "Failed to backup /etc/apt/sources.list"
    
    sudo sed -i "s|http://[^ ]*ubuntu.com/ubuntu/|$mirror_url|g" /etc/apt/sources.list || {
        sudo mv /etc/apt/sources.list.backup /etc/apt/sources.list
        error_exit "Failed to update /etc/apt/sources.list with mirror: $mirror_url"
    }
}

# Trap to ensure sources.list is restored on exit
trap restore_sources_list EXIT

# Function to select a working mirror
select_working_mirror() {
    local all_mirrors=("$PRIMARY_MIRROR" "${MIRRORS[@]}")
    for mirror in "${all_mirrors[@]}"; do
        echo "Trying mirror: $mirror"
        update_sources_list "$mirror"
        if run_with_retries "apt-get update" "$MAX_RETRIES" sudo apt-get update; then
            #shellcheck disable=SC2034
            CURRENT_MIRROR="$mirror"
            echo "Successfully updated package list using mirror: $mirror"
            return 0
        else
            echo "Failed to update package list using mirror: $mirror"
            sudo mv /etc/apt/sources.list.backup /etc/apt/sources.list
            continue
        fi
    done
    error_exit "All mirrors failed to update package list."
}

# Start of the script
echo "=== Docker Installation Script with Mirror Fallback ==="

# Select a working mirror
select_working_mirror

# Install prerequisites with retries
echo "Installing prerequisites..."
run_with_retries "Install prerequisites" "$MAX_RETRIES" sudo apt-get install -y apt-transport-https ca-certificates curl gnupg lsb-release || error_exit "Failed to install prerequisites"

# Add Docker's official GPG key with proper handling of existing file
echo "Adding Docker's GPG key..."
if [ -f "/usr/share/keyrings/docker-archive-keyring.gpg" ]; then
    echo "Docker GPG key file already exists, removing old file..."
    sudo rm -f /usr/share/keyrings/docker-archive-keyring.gpg || error_exit "Failed to remove existing Docker GPG key"
fi

run_with_retries "Add Docker GPG key" "$MAX_RETRIES" \
sh -c 'curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --batch --yes --dearmor -o /usr/share/keyrings/docker-archive-keyring.gpg' \
|| error_exit "Failed to add Docker GPG key"

# Set up the Docker stable repository
echo "Setting up Docker repository..."
echo \
"deb [arch=amd64 signed-by=/usr/share/keyrings/docker-archive-keyring.gpg] https://download.docker.com/linux/ubuntu \
$(lsb_release -cs) stable" | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null || error_exit "Failed to add Docker repository"

# Update package index again with retries
echo "Updating package list after adding Docker repository..."
run_with_retries "apt-get update after Docker repo" "$MAX_RETRIES" sudo apt-get update || error_exit "Failed to update package list after adding Docker repository"

# Install Docker Engine with retries
echo "Installing Docker Engine..."
run_with_retries "Install Docker Engine" "$MAX_RETRIES" sudo apt-get install -y docker-ce docker-ce-cli containerd.io || error_exit "Failed to install Docker Engine"

# Start Docker service with retries
echo "Starting Docker service..."
run_with_retries "Start Docker service" "$MAX_RETRIES" sudo systemctl start docker || error_exit "Failed to start Docker service"

# Enable Docker to start on boot with retries
echo "Enabling Docker to start on boot..."
run_with_retries "Enable Docker service" "$MAX_RETRIES" sudo systemctl enable docker || error_exit "Failed to enable Docker service"

# Remove the trap since the script succeeded
trap - EXIT

exit 0
