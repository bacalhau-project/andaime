#!/bin/bash
set -e

# Clean and recreate mock directories
rm -rf mocks/*
mkdir -p mocks/{aws,azure,common,gcp,sshutils}

# Initialize go.mod files for each mock directory
for dir in aws azure common gcp sshutils; do
  cd mocks/$dir
  echo "module github.com/bacalhau-project/andaime/mocks/$dir" > go.mod
  echo -e "\ngo 1.21\n\nrequire (\n  github.com/bacalhau-project/andaime v0.0.0\n)\n\nreplace github.com/bacalhau-project/andaime => ../.." >> go.mod
  cd ../..
done

# Update go modules
go mod tidy
go mod vendor

# Generate mocks and run tests
just genmock
just test
