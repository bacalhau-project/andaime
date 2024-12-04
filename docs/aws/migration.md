# Migration Guide: CDK to Direct AWS SDK

This guide helps you migrate from the AWS CDK-based deployment to the new direct AWS SDK implementation.

## Key Changes

### 1. Configuration Updates
Previous CDK configuration:
```yaml
aws:
  cdk_stack_name: "MyStack"
  cdk_app: "app.ts"
```

New SDK configuration:
```yaml
aws:
  account_id: "123456789012"
  region: "us-west-2"
  default_machine_type: "t3.micro"
  default_disk_size_gb: 30
```

### 2. Resource Creation
- VPC and networking components are now created directly via AWS SDK
- EC2 instances are provisioned without CDK constructs
- Resource polling replaces CloudFormation stack events

### 3. Error Handling
- More granular error handling with specific error types
- Improved recovery mechanisms
- Detailed logging of operations

## Migration Steps

1. Update Configuration
   - Remove CDK-specific configuration
   - Add new SDK configuration parameters
   - Update machine specifications

2. Code Changes
   - Remove CDK imports and dependencies
   - Update resource creation calls
   - Update error handling

3. Testing
   - Run integration tests
   - Verify resource creation
   - Check cleanup processes

## Rollback Procedure

If issues occur during migration:

1. Stop the deployment
2. Run cleanup to remove resources
3. Revert to CDK version
4. Restore original configuration

## Support

For migration assistance:
- File issues on GitHub
- Contact the development team
- Check documentation updates
