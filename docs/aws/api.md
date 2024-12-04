# AWS Provider API Documentation

## Overview
The AWS Provider implements direct AWS SDK resource provisioning for EC2 instances, networking, and associated resources. This document details the public interfaces and configuration options.

## Core Interfaces

### AWSProvider
The main provider struct that handles AWS resource provisioning:

```go
type AWSProvider struct {
    AccountID       string
    Config          *aws.Config
    Region          string
    ClusterDeployer common_interface.ClusterDeployerer
    UpdateQueue     chan display.UpdateAction
    VPCID           string
    EC2Client       aws_interface.EC2Clienter
}
```

### Key Methods

#### NewAWSProvider
```go
func NewAWSProvider(accountID, region string) (*AWSProvider, error)
```
Creates a new AWS provider instance with the specified account ID and region.

#### CreateInfrastructure
```go
func (p *AWSProvider) CreateInfrastructure(ctx context.Context) error
```
Creates the core AWS infrastructure including VPC, subnets, internet gateway, and routing tables.

#### CreateVPC
```go
func (p *AWSProvider) CreateVPC(ctx context.Context) error
```
Creates a VPC with the following components:
- VPC with CIDR block 10.0.0.0/16
- Public and private subnets
- Internet gateway
- Route tables for internet access

#### StartResourcePolling
```go
func (p *AWSProvider) StartResourcePolling(ctx context.Context) error
```
Begins polling AWS resources to monitor their state and update the deployment status.

## Error Handling
The provider implements comprehensive error handling with detailed error types and recovery mechanisms:

- Network errors include retries with exponential backoff
- Resource creation failures trigger automatic cleanup
- API throttling is handled with rate limiting
- Detailed error messages are logged for debugging

## Logging
Structured logging is implemented using zap logger with the following levels:
- DEBUG: Detailed debugging information
- INFO: General operational information
- WARN: Warning messages for potential issues
- ERROR: Error conditions that need attention

## Resource Management
Resources are tagged with:
- Name
- Project ID
- Deployment ID
- Creation timestamp

This enables easy resource tracking and cleanup.
