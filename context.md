# Andaime Project Context

Copyright (c) 2024 Expanso, Inc., oss@expanso.io
Licensed under the Apache License, Version 2.0
File path: context.md

## Project Overview

Andaime is an open-source project aimed at creating a robust and user-friendly tool for deploying and managing Bacalhau clusters across multiple cloud providers. The project primarily uses Go (Golang) for both its CLI and backend services.

## Technology Stack

### Backend
- Go (Golang) for CLI and backend services
- Azure SDK for Go
- Google Cloud SDK for Go
- AWS SDK for Go (if implemented)

### Infrastructure
- Azure Cloud
- Google Cloud Platform
- AWS (if implemented)

### Development Tools
- Go testing framework for backend testing
- GitHub Actions for CI/CD

## Project Structure
andaime/
├── cmd/ # Command-line applications
│ ├── beta/ # Beta commands
│ │ ├── azure/ # Azure-specific commands
│ │ └── gcp/ # GCP-specific commands
├── internal/ # Internal packages
│ ├── clouds/ # Cloud-specific implementations
│ │ ├── azure/ # Azure-specific code
│ │ ├── gcp/ # GCP-specific code
│ │ └── general/ # General cloud utilities
│ └── testdata/ # Test data and mocks
├── pkg/ # Public packages
│ ├── display/ # Display utilities
│ ├── logger/ # Logging utilities
│ ├── models/ # Data models
│ ├── providers/ # Cloud provider implementations
│ │ ├── azure/ # Azure provider
│ │ ├── gcp/ # GCP provider
│ │ └── common/ # Common provider utilities
│ ├── sshutils/ # SSH utilities
│ └── utils/ # General utilities
├── tests/ # Integration tests
└── vendor/ # Vendored dependencies


## Key Components

1. Cluster Deployment: Core functionality for deploying Bacalhau clusters on different cloud providers.
2. Resource Management: Creating, updating, and deleting cloud resources (VMs, networks, etc.).
3. SSH Management: Utilities for SSH connections to deployed machines.
4. Configuration Management: Handling of configuration files for different deployments.
5. CLI Interface: Command-line interface for interacting with the tool.
6. Cloud Provider Abstraction: Common interfaces for different cloud providers.

## Development Guidelines

1. Follow idiomatic Go practices.
2. Use interfaces for abstraction, especially for cloud provider implementations.
3. Implement comprehensive unit tests for all packages.
4. Use meaningful variable and function names that clearly describe their purpose.
5. Handle errors appropriately and provide informative error messages.
6. Use context for cancellation and timeouts in long-running operations.
7. Implement logging throughout the application for debugging and monitoring.

## Next Steps

1. Implement remaining cloud provider support (e.g., AWS if not already implemented).
2. Enhance error handling and recovery mechanisms.
3. Improve test coverage, especially for edge cases and error scenarios.
4. Implement more robust logging and monitoring capabilities.
5. Enhance the CLI with more features and improved user experience.
6. Develop comprehensive documentation for setup, usage, and contribution guidelines.
7. Implement continuous integration and deployment pipelines.
8. Conduct security audits and implement necessary security measures.

This context provides a high-level overview of the Andaime project. As development progresses, this document should be updated to reflect the current state of the project, including more detailed information about specific components, architectural decisions, and development milestones.