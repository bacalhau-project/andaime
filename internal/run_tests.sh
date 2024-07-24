#!/usr/bin/env bash


/usr/local/bin/flox activate -r "aronchick/andaime" -t -- go test ./...

# Produce a report of the test coverage, but do it in the background. Overwrite any file for "coverage.out" in the root directory.
/usr/local/bin/flox activate -r "aronchick/andaime" -t -- go test -coverprofile=coverage.out ./... &

