# Default recipe (runs when you just type 'just')
default:
    @just --list

# Generate mocks
genmock:
    @echo "Generating mocks..."
    @mockery --all --dir=./pkg/providers --outpkg=mocks
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
    go build -o build/andaime ./cmd/andaime

# Build release artifacts for multiple platforms
build-release:
    mkdir -p dist
    GOOS=linux GOARCH=amd64 go build -o dist/andaime_linux_amd64 ./cmd/andaime
    GOOS=darwin GOARCH=amd64 go build -o dist/andaime_darwin_amd64 ./cmd/andaime
    GOOS=darwin GOARCH=arm64 go build -o dist/andaime_darwin_arm64 ./cmd/andaime
    GOOS=windows GOARCH=amd64 go build -o dist/andaime_windows_amd64.exe ./cmd/andaime
