# GCP Deployment Optimization SOP

## 1. Performance Analysis

### Current Flow Issues
1. Sequential resource creation
2. Frequent display updates
3. Incomplete resource status tracking
4. Network connectivity issues
5. Timing accuracy problems

### Bottlenecks Identified
1. API enablement is sequential and slow
2. VM creation is not parallelized efficiently
3. Display updates cause unnecessary overhead
4. Firewall rules are not comprehensive
5. Resource completion tracking is imprecise

## 2. Optimization Strategy

### 2.1 Parallelization Improvements
- Group API enablement into batches
- Parallelize VM creation with proper error handling
- Implement concurrent firewall rule creation
- Add resource dependency tracking for optimal ordering

### 2.2 Display Optimization
- Batch display updates with 500ms buffer
- Implement circular buffer for status updates
- Add proper resource completion tracking
- Reduce unnecessary refreshes

### 2.3 Network Configuration
- Ensure bidirectional port access:
  - SSH (22)
  - Bacalhau API (1234)
  - Bacalhau P2P (1235)
  - NATS (4222)
- Create comprehensive firewall rules for all required ports
- Implement proper network testing

### 2.4 Resource Tracking
- Add explicit resource completion flags
- Implement proper state machine for resources
- Track dependencies between resources
- Add validation for resource completion

## 3. Implementation Plan

### Phase 1: Display Optimization
1. Add display update batching
2. Implement proper resource completion tracking
3. Fix timing issues

### Phase 2: Network Configuration
1. Update firewall rules for all required ports
2. Add network connectivity validation
3. Implement proper network testing

### Phase 3: Parallelization
1. Implement concurrent API enablement
2. Add parallel VM creation
3. Optimize resource creation ordering

## 4. Validation Steps

### 4.1 Performance Testing
1. Measure deployment time before and after changes
2. Monitor resource creation timing
3. Validate display update frequency
4. Test network connectivity

### 4.2 Network Validation
1. Test all required ports
2. Verify bidirectional communication
3. Validate firewall rules
4. Check service connectivity

### 4.3 Resource Completion
1. Verify resource status tracking
2. Test completion timing accuracy
3. Validate state transitions
4. Check dependency handling

## 5. Implementation Details

### 5.1 Display Updates
```go
// Batch updates for 500ms
type BatchedDisplayUpdate struct {
    updates []models.DisplayStatus
    timer   *time.Timer
}

// Process updates in batches
func (b *BatchedDisplayUpdate) Add(update models.DisplayStatus) {
    b.updates = append(b.updates, update)
    if b.timer == nil {
        b.timer = time.AfterFunc(500*time.Millisecond, b.flush)
    }
}
```

### 5.2 Network Configuration
```go
// Required ports for bidirectional communication
var RequiredPorts = []struct {
    Port        int
    Protocol    string
    Description string
}{
    {22, "tcp", "SSH"},
    {1234, "tcp", "Bacalhau API"},
    {1235, "tcp", "Bacalhau P2P"},
    {4222, "tcp", "NATS"},
}

// Create bidirectional firewall rules
func createBidirectionalRules(ctx context.Context, projectID string) error {
    for _, port := range RequiredPorts {
        // Create ingress rule
        if err := createFirewallRule(ctx, projectID, port, "INGRESS"); err != nil {
            return err
        }
        // Create egress rule
        if err := createFirewallRule(ctx, projectID, port, "EGRESS"); err != nil {
            return err
        }
    }
    return nil
}
```

### 5.3 Resource Completion
```go
// Track resource completion
type ResourceStatus struct {
    Complete bool
    Time     time.Time
    Error    error
}

// Update resource status
func (m *Machine) UpdateResourceStatus(resource string, complete bool, err error) {
    m.resourceStatus[resource] = ResourceStatus{
        Complete: complete,
        Time:     time.Now(),
        Error:    err,
    }
}
```

## 6. Testing and Validation

### 6.1 Performance Tests
1. Run deployment with timing instrumentation
2. Monitor resource creation parallelization
3. Verify display update frequency
4. Test network connectivity

### 6.2 Network Tests
1. Test all ports bidirectionally
2. Verify firewall rule creation
3. Validate service communication
4. Check connectivity between nodes

### 6.3 Resource Tests
1. Verify resource completion tracking
2. Test state transitions
3. Validate dependency handling
4. Check timing accuracy

## 7. Rollout Plan

### 7.1 Phase 1 - Display Updates
1. Implement display batching
2. Add resource completion tracking
3. Fix timing issues
4. Test and validate

### 7.2 Phase 2 - Network Configuration
1. Update firewall rules
2. Add network validation
3. Test connectivity
4. Verify services

### 7.3 Phase 3 - Parallelization
1. Implement concurrent API enablement
2. Add parallel VM creation
3. Test and validate
4. Monitor performance

## 8. Monitoring and Maintenance

### 8.1 Performance Monitoring
1. Track deployment times
2. Monitor resource creation timing
3. Check display update frequency
4. Verify network performance

### 8.2 Issue Resolution
1. Monitor error rates
2. Track resource creation failures
3. Check network connectivity issues
4. Verify resource completion accuracy

## 9. Success Criteria

1. Deployment time reduced by 50%
2. Display updates limited to 500ms intervals
3. All required ports properly configured
4. Resource completion tracking accurate
5. Network connectivity verified
