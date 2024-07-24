#!/usr/bin/env bash

. <( /usr/local/bin/flox activate; );

# Check if Go is installed
if ! command -v go &> /dev/null
then
    echo "Error: Go is not installed or not in PATH"
    echo "Please install Go from https://golang.org/doc/install"
    echo "After installation, make sure 'go' is in your PATH"
    echo "Once Go is installed, run this script again to execute the tests"
    exit 1
fi

# Set GOPATH to an absolute path
export GOPATH="$HOME/go"

# Run the tests
go test ./...

# Produce a report of the test coverage, but do it in the background. Overwrite any file for "coverage.out" in the root directory.
go test -coverprofile=coverage.out ./... &

echo "Tests completed. Coverage report generated in coverage.out"

