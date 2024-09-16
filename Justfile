# Default recipe (runs when you just type 'just')
default:
    @just --list

# Generate mocks
genmock:
    @echo "Generating mocks..."
    @docker run --rm -v "${PWD}:/src" -w /src vektra/mockery --all --keeptree --with-expecter
    @echo "Mocks generated successfully."
