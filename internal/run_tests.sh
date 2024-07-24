#!/usr/bin/env bash

# Set GOPATH to an absolute path
export GOPATH="$HOME/go"

# Run the tests
go test ./...

# Produce a report of the test coverage, but do it in the background. Overwrite any file for "coverage.out" in the root directory.
go test -coverprofile=coverage.out ./... &

