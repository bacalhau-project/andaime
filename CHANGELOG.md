# Andaime Changelog

## [Unreleased]

### 🚀 Features

#### GCP Provider Enhancements
- 🌐 **Firewall Rules Configuration**
  - Added comprehensive firewall rules for SSH, Bacalhau, and NATS ports
  - Implemented dynamic port configuration
  ```go
  func CreateFirewallRules(projectID string, ports []int) error {
    // New implementation with dynamic port handling
    for _, port := range ports {
      createFirewallRule(projectID, port)
    }
  }
  ```

- 🔍 **IP Allocation and Metrics**
  - Enhanced logging for IP provisioning
  - Added detailed metrics tracking
  ```go
  type IPAllocationMetrics struct {
    AttemptCount   int
    FailureCount   int
    LatencyMs      float64
    RegionStats    map[string]*RegionMetrics
  }
  ```

#### Deployment Improvements
- 🚦 **Provisioning Workflow**
  - Added more resilient deployment command
  - Improved error handling and retry mechanisms
  ```bash
  # New provision command with enhanced error tracking
  andaime provision --provider gcp --retry-attempts 3
  ```

#### CI/CD Enhancements
- 🤖 **GitHub Actions Workflow**
  - Implemented comprehensive CI/CD pipeline
  - Added multi-platform build and release process
  ```yaml
  - name: Build Release Artifacts
    run: |
      GOOS=linux GOARCH=amd64 go build -o dist/andaime_linux_amd64
      GOOS=darwin GOARCH=arm64 go build -o dist/andaime_darwin_arm64
  ```

### 🐛 Bug Fixes

#### GCP Provider
- 🔧 **Resource Management**
  - Fixed zone validation logic
  - Corrected resource state initialization
  ```go
  func validateGCPZone(zone string) error {
    // Improved zone validation with better error messages
    if !isValidZone(zone) {
      return fmt.Errorf("invalid GCP zone: %s", zone)
    }
  }
  ```

- 🌐 **Network Configuration**
  - Resolved IP address association issues
  - Improved network and firewall rule creation

#### Deployment Fixes
- 🔒 **SSH and Service Tracking**
  - Enhanced SSH service status tracking
  - Corrected machine resource state management

### 🔧 Refactoring

#### Provider Abstraction
- 🏗️ **Infrastructure as Code**
  - Replaced AWS CDK with direct AWS SDK provisioning
  - Simplified provider interfaces
  ```go
  type CloudProvider interface {
    CreateVM(config VMConfig) (*VM, error)
    ConfigureNetwork(networkSpec NetworkSpec) error
  }
  ```

#### Code Structure
- 🧩 **Deployment Workflow**
  - Simplified step registration
  - Improved error handling patterns
  ```go
  func (r *StepRegistry) RegisterStep(step ProvisioningStep, message StepMessage) {
    // More flexible step registration
    r.steps[step] = message
  }
  ```

### 🚨 Breaking Changes
- Significant changes to GCP provider implementation
- Refactored deployment and provisioning logic
- Updated build and release processes

### 📦 Dependency Updates
- Upgraded Go to 1.21
- Removed AWS CDK dependencies
- Updated GitHub Actions workflow configurations

## [Previous Versions]
- For changelog of previous versions, please refer to GitHub Releases
```
