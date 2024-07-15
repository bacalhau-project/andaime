# Define the project name and Go files
PROJECT_NAME := andaime
GO_FILES := andaime.go

# Define the output directories
BUILD_DIR := build
RELEASE_BASE_DIR := release

# Define the architectures and operating systems to build for
ARCHS := amd64 arm64
OSES := linux darwin

# Create the base release directory
$(RELEASE_BASE_DIR):
	mkdir -p $(RELEASE_BASE_DIR)

# Build for the local platform
.PHONY: build
build:
	go build -o $(BUILD_DIR)/$(PROJECT_NAME)

# Build for all specified architectures and operating systems and zip the binaries
.PHONY: release
release: $(RELEASE_BASE_DIR)
	@ANDAIME_VERSION=$$(./$(BUILD_DIR)/$(PROJECT_NAME) version | grep -oE '[0-9]+\.[0-9]+\.[0-9]+'); \
	RELEASE_DIR=$(RELEASE_BASE_DIR)/$$ANDAIME_VERSION; \
	mkdir -p $$RELEASE_DIR; \
	for os in $(OSES); do \
		for arch in $(ARCHS); do \
			OUTPUT_FILE=$$RELEASE_DIR/$(PROJECT_NAME)-$$os-$$arch.tar.gz; \
			GOOS=$$os GOARCH=$$arch go build -o $$RELEASE_DIR/$(PROJECT_NAME)-$$os-$$arch; \
			tar -czvf $$OUTPUT_FILE -C $$RELEASE_DIR $(PROJECT_NAME)-$$os-$$arch; \
			rm $$RELEASE_DIR/$(PROJECT_NAME)-$$os-$$arch; \
		done; \
	done

# Clean the release directory
.PHONY: clean_release
clean_release:
	rm -rf $(RELEASE_BASE_DIR)

# Clean the build and release directories
.PHONY: clean
clean:
	rm -rf $(BUILD_DIR) $(RELEASE_BASE_DIR)
