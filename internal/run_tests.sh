#!/usr/bin/env bash

# Check if Go is installed
if ! command -v go &> /dev/null
then
    echo "Error: Go is not installed or not in PATH"
    exit 1
fi

# Set GOPATH to an absolute path
export GOPATH="$HOME/go"

# Run the tests
go test ./...

# Produce a report of the test coverage, but do it in the background. Overwrite any file for "coverage.out" in the root directory.
go test -coverprofile=coverage.out ./... &

echo "Tests completed. Coverage report generated in coverage.out"

