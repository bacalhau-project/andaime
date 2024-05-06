# Andaime
## A CLI tool for generating an MVP for running a private Bacalhau cluster

## Usage

```
./andaime 
```

## Flags

```
  -project-name <string>
    	Set project name (default: "bacalhau-by-andaime")
  -target-platform <string>
    	Set target platform "aws"/"gcp"/"azure" (default "gcp")
  -orchestrator-nodes <int>
    	Set number of orchestrator nodes (default: 1)
  -compute-nodes <int>
    	Set number of compute nodes (default: 2)
  -verbose <bool>
    	Generate verbose output throughout execution (default: false)
```