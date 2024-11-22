# Node Provisioning Standard Operating Procedure

## Overview
This SOP defines the standard process for provisioning nodes and tracking their deployment status.

## Node Information Tracking
Each node deployment must track:
- Node name
- Node IP address
- Orchestrator IP (if applicable)
- Docker installation status
- Bacalhau installation status
- Custom script installation status
- SSH username
- SSH private key path

## Display Format
Progress display must follow this format:
```
🚀 Starting node provisioning process                              [   0%]
    ✅ SSH connection established successfully  (<IP>)   
🏠 Provisioning base packages...                                   [  10%]
    ✅ Base system provisioned successfully (<IP>)                        
...
✅ Successfully provisioned node on <IP>                           [ 100%]
```

## Storage
- All node information must be stored in `nodes.yml`
- File should be created if it doesn't exist
- New nodes should be appended to existing file
- Each node entry must include all tracked information
- YAML format must be used for easy reading and parsing

## Progress Tracking
- Each major step must show both start and completion messages
- Progress percentage must be shown in fixed-width format [  0%]
- Completion messages must be indented and include relevant IPs
- Status emojis must be used consistently

## Implementation Details
1. Use structured ProvisioningStep type for each step
2. Maintain atomic YAML file operations
3. Validate all required fields before writing
4. Handle concurrent access to nodes.yml safely
5. Provide clear error messages for missing information

## Error Handling
- Failed steps must be clearly marked with ❌
- Error details must be logged
- Partial deployments must be marked as incomplete in nodes.yml
