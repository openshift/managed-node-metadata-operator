# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is the Managed Node Metadata Operator for OpenShift Dedicated - a Kubernetes operator written in Go that syncs metadata (labels and taints) from MachineSets to existing Nodes. The operator watches MachineSet objects and reconciles any labels or taints to corresponding Nodes within the pool, solving the limitation where machine-api-operator doesn't reconcile metadata to existing machines.

## Key Architecture

- **Controller**: `controllers/machineset_controller.go` - Main controller that watches MachineSets and reconciles metadata to Machines and their referenced Nodes
- **Core Logic**: `pkg/machine/machine.go` - Contains the core reconciliation logic for syncing labels and taints
- **Metrics**: `pkg/metrics/metrics.go` - Prometheus metrics for operator monitoring
- **Main Entry**: `main.go` - Operator entry point using controller-runtime

The operator follows this reconciliation flow:
1. Watch MachineSet changes
2. Loop through all Machines in the MachineSet
3. Sync label/taint changes from MachineSet to Machine
4. Get referenced Node and sync changes from Machine to Node

## Development Commands

This project uses OpenShift's golang-osd-operator boilerplate convention. Key commands:

### Local Development
```bash
# Run operator locally (uses your current kubeconfig)
go run .

# Run unit tests
make test

# Run integration tests (requires cluster and env vars)
go test -count=1 ./int/

# Run linting and static analysis
make lint

# Run code generation and validation
make validate

# Build the binary
make go-build
```

### Testing Requirements
For E2E/integration tests, these environment variables must be set:
- `OCM_TOKEN` - OCM authentication token
- `OCM_CLUSTER_ID` - Target cluster ID
- `OCM_ENV` - Environment (stage|int)

### Container/Cluster Development
```bash
# Build and push operator image
make docker-build docker-push

# Deploy to cluster (requires IMAGE_* env vars set)
make deploy

# Run tests in container environment (mirrors CI)
make container-test
make container-lint
make container-validate
```

### Required Environment Variables for Development
```bash
export REGISTRY_TOKEN=...  # quay.io encrypted password
export REGISTRY_USER=...   # quay.io username
export IMG=quay.io/$REGISTRY_USER/managed-node-metadata-operator
export IMAGE_REPOSITORY=$REGISTRY_USER
```

## FIPS Support

This project has FIPS enabled (`FIPS_ENABLED=true` in Makefile). The `fips.go` file is auto-generated to ensure FIPS-compliant TLS configuration.

## Testing

- Unit tests: `make test` (excludes integration tests in `./int/`)
- Integration tests: `go test ./int/` (requires cluster access and OCM env vars)
- E2E tests: Located in `test/e2e/` with specific OCM requirements
- Container tests: `make container-test` for CI-like environment

The project uses Ginkgo/Gomega for testing framework.