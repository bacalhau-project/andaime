# Andaime Refactoring Standard Operating Procedure

## Overview
This SOP outlines the step-by-step process for refactoring the Andaime codebase to reduce duplication across AWS, GCP, and Azure implementations, particularly in tests and mocking. Each step is designed to maintain test coverage while simplifying the codebase.

## Prerequisites
- All tests passing in current state
- No pending changes in working directory
- Access to all cloud provider test configurations

## Steps

### 1. Create Common Test Utilities (2 AGU)
- Create `pkg/testutil/ssh_behavior.go`
  - Extract common SSH mocking patterns
  - Create reusable SSH behavior builders
  - Add helper functions for common SSH test scenarios
- Create `pkg/testutil/deployment_verification.go`
  - Extract shared deployment verification logic
  - Create standard verification helpers

### 2. Consolidate Provider Test Setup (3 AGU)
- Create `pkg/testutil/provider_test_suite.go`
  - Extract common test suite setup logic
  - Create base test suite with shared functionality
  - Add provider-specific test suite extensions
- Update existing provider tests to use new base suite
  - AWS provider tests
  - GCP provider tests
  - Azure provider tests

### 3. Standardize Resource Mocking (3 AGU)
- Create `pkg/testutil/resource_mocks.go`
  - Define common resource mock interfaces
  - Implement shared mock builders
  - Add helper functions for standard mock scenarios
- Update provider tests to use standardized mocks
  - AWS EC2 client mocks
  - GCP compute client mocks
  - Azure resource client mocks

### 4. Implement Common Test Fixtures (2 AGU)
- Create `pkg/testutil/fixtures.go`
  - Define shared test data structures
  - Implement fixture factories
  - Add helper functions for test data generation
- Update tests to use common fixtures
  - Machine configurations
  - Network configurations
  - Deployment configurations

### 5. Consolidate Integration Tests (4 AGU)
- Create `test/integration/common_test.go`
  - Extract shared integration test patterns
  - Define common test scenarios
  - Implement provider-agnostic test helpers
- Update provider integration tests
  - Refactor AWS integration tests
  - Refactor GCP integration tests
  - Refactor Azure integration tests

### 6. Standardize Mock Expectations (3 AGU)
- Create `pkg/testutil/expectations.go`
  - Define common mock expectations
  - Create expectation builders
  - Add verification helpers
- Update provider tests with standardized expectations
  - SSH operation expectations
  - Resource creation expectations
  - Service deployment expectations

### 7. Clean Up and Documentation (3 AGU)
- Remove duplicate test code
- Update documentation
- Add examples for new test utilities
- Create migration guide

## Validation
Each step must pass:
1. All existing tests
2. Linting checks
3. Compilation without warnings
4. No reduction in test coverage

## Rollback Plan
1. Each step has a corresponding git branch
2. Revert to previous state if validation fails
3. Document issues in rollback for future attempts

## Success Criteria
1. Reduced code duplication
2. Simplified test maintenance
3. Maintained test coverage
4. Improved code organization
5. Clear documentation
