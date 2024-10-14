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