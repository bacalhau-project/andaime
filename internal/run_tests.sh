#!/usr/bin/env bash

# Check if Go is installed
if ! command -v go &> /dev/null
then
    echo "Error: Go is not installed or not in PATH"
    echo "Please install Go from https://golang.org/doc/install"
    echo "After installation, make sure 'go' is in your PATH"
    echo "Once Go is installed, run this script again to execute the tests"
    echo ""
    echo "For macOS users:"
    echo "1. Install Homebrew if not already installed: https://brew.sh/"
    echo "2. Run: brew install go"
    echo "3. Add Go to your PATH by adding this line to your ~/.zshrc or ~/.bash_profile:"
    echo "   export PATH=\$PATH:/usr/local/go/bin"
    echo "4. Reload your shell configuration with: source ~/.zshrc (or ~/.bash_profile)"
    exit 1
fi

# Set GOPATH to an absolute path
export GOPATH="${HOME}/go"

# Run the tests
go test ./...

# Produce a report of the test coverage, but do it in the background. Overwrite any file for "coverage.out" in the root directory.
go test -coverprofile=coverage.out ./... &

echo "Tests completed. Coverage report generated in coverage.out"

