#!/usr/bin/env bash

. <( /usr/local/bin/flox activate; );

# Check if Go is installed
if ! command -v go &> /dev/null
then
    echo "Error: Go is not installed or not in PATH"
    exit 1
fi

# Set GOPATH to an absolute path
export GOPATH="${HOME}/go"

# Print out information letting the user know that testing is beginning
echo "Running tests..." 

# Run the tests
go test -v ./...

# Produce a report of the test coverage, but do it in the background. Overwrite any file for "coverage.out" in the root directory.
nohup go test -coverprofile=coverage.out ./... > coverage.log 2>&1 &

echo "Tests completed. Coverage report is being generated in the background."