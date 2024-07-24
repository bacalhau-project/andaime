#!/usr/bin/env bash

# Enter the Nix shell and run the tests
nix-shell --run "go version && go test ./..."
