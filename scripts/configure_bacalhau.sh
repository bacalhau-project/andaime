#!/bin/bash

# Install Bacalhau if not already installed
if ! command -v bacalhau &> /dev/null; then
    curl -sL https://get.bacalhau.org/install.sh | bash
fi

# Execute Bacalhau configuration commands
for cmd in "$@"
do
    bacalhau config set $cmd
done
