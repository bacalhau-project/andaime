# Default recipe (runs when you just type 'just')
default:
    @just --list

# Generate mocks
genmock:
    @echo "Generating mocks..."
    @mockery --all --dir=./pkg/models/interfaces --recursive --outpkg=mocks
    @echo "Mocks generated successfully."

# Generate cloud data
gencloud:
    @go run ./internal/generate_cloud_data.go

# Run tests
test:
    go test ./...

# Build binary
build:
    mkdir -p build
    go build -o build/andaime .

# Build release artifacts for multiple platforms
build-release:
    mkdir -p dist
    GOOS=linux GOARCH=amd64 go build -o dist/andaime_linux_amd64 .
    GOOS=darwin GOARCH=amd64 go build -o dist/andaime_darwin_amd64 .
