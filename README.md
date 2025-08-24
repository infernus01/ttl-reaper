# TTL Reaper Controller

A universal Kubernetes controller that automates the cleanup of custom resources based on TTL (Time To Live) policies. Unlike resource-specific solutions, TTL Reaper provides a generic, declarative approach to resource lifecycle management that works with any Kubernetes custom resource.

## 📚 Documentation

- **[📋 Project Overview](PROJECT_OVERVIEW.md)** - Executive summary and comprehensive project documentation
- **[🎯 Problem Statement](PROBLEM_STATEMENT.md)** - Detailed analysis of the problem we're solving and our solution approach  
- **[⚖️ Comparison with Tektoncd-Pruner](COMPARISON_WITH_TEKTONCD_PRUNER.md)** - How TTL Reaper differs from and improves upon existing solutions
- **[🔧 Technical Deep Dive: main.go](main_go_explained.txt)** - Line-by-line explanation of the controller manager
- **[🧹 Technical Deep Dive: controller](ttlreaper_controller_explained.txt)** - Detailed explanation of the reconciliation logic

## Overview

The TTL Reaper Controller monitors custom resources that have a `spec.ttlSecondsAfterFinished` field and automatically deletes them after the specified time has elapsed since completion. This is particularly useful for cleaning up resources like Tekton PipelineRuns, TaskRuns, or any other custom resources that accumulate over time.

**Architecture:** TTLReaperConfig is cluster-scoped, allowing platform administrators to define cleanup policies that can target any namespace in the cluster. The controller runs in the `ttl-reaper-system` namespace but operates cluster-wide.

## ✨ Key Features

- **🌍 Universal**: Works with any custom resource (PipelineRuns, Jobs, CRDs, etc.)
- **⚡ Real-time**: Continuous reconciliation, not batch processing
- **📋 Declarative**: Simple CRD-based configuration
- **🔧 Extensible**: Add new resource types without code changes
- **🛡️ Production-ready**: Built with controller-runtime best practices
- **🎯 Cluster-scoped**: Centralized policy management

## Quick Start

### Prerequisites

- [ko](https://ko.build/) for building and deploying (install with `go install github.com/google/ko@latest`)
- [kind](https://kind.sigs.k8s.io/) for local testing (install with `go install sigs.k8s.io/kind@latest`)
- kubectl configured to access your cluster
- For production: Kubernetes cluster (v1.19+)

### Local Development (Recommended)

1. **Deploy to a local kind cluster:**
   ```bash
   make kind-deploy
   ```

2. **Clean up:**
   ```bash
   make kind-delete
   ```

### Production Installation

1. **Install the CRD:**
   ```bash
   make install
   ```

2. **Deploy the controller:**
   ```bash
   make ko-apply
   ```

3. **Create a TTL Reaper configuration:**
   ```bash
   kubectl apply -f examples/tekton-cleanup.yaml
   ```

### Building and Running Locally

1. **Build the project:**
   ```bash
   make build
   ```

2. **Run locally (requires kubeconfig):**
   ```bash
   make run
   ```

3. **Build container image with ko:**
   ```bash
   make ko-build
   ```

## Configuration

The TTL Reaper Controller is configured using `TTLReaperConfig` custom resources. Here's an example:

```yaml
apiVersion: ttlreaper.io/v1alpha1
kind: TTLReaperConfig
metadata:
  name: tekton-pipelinerun-cleanup

spec:
  # Target namespace (required for cluster-scoped configs)
  targetNamespace: tekton-pipelines
  
  # Kind of resources to monitor
  targetKind: PipelineRun
  
  # API version of the target kind
  targetApiVersion: tekton.dev/v1beta1
  
  # Path to TTL field (optional, defaults to spec.ttlSecondsAfterFinished)
  ttlFieldPath: "spec.ttlSecondsAfterFinished"
  
  # Check interval in seconds (optional, defaults to 300)
  checkInterval: 300
```

### Configuration Fields

| Field | Description | Required | Default |
|-------|-------------|----------|---------|
| `targetNamespace` | the namespace where the target resources exist | Yes | - |
| `targetKind` | the kind of custom resources to monitor for TTL cleanup | Yes | - |
| `targetApiVersion` | the API version of the target kind | Yes | - |
| `ttlFieldPath` | the path to the TTL field in the target resource spec | No | `spec.ttlSecondsAfterFinished` |
| `checkInterval` | how often to check for expired resources (in seconds) | No | 300 |

## How It Works

1. **Resource Monitoring**: The controller watches `TTLReaperConfig` resources for configuration changes.

2. **Target Resource Discovery**: For each configuration, it periodically lists resources of the specified kind in the target namespace.

3. **TTL Evaluation**: For each resource found:
   - Checks if the resource has the specified TTL field
   - Determines if the resource has finished execution (based on status conditions or completion time)
   - Calculates if the TTL has expired since completion

4. **Cleanup**: Deletes resources that have exceeded their TTL after completion.

## Resource Completion Detection

The controller determines if a resource has "finished" by checking:

1. **Status Conditions**: Looks for conditions with `type: "Succeeded"` and `status: "True"`
2. **Completion Time**: Checks for `status.completionTime` field
3. **Condition Timestamps**: Uses `lastTransitionTime` from completion conditions as fallback

## Examples

### Tekton PipelineRun Cleanup

```yaml
# TTL Reaper Configuration
apiVersion: ttlreaper.io/v1alpha1
kind: TTLReaperConfig
metadata:
  name: tekton-pipelinerun-cleanup

spec:
  targetKind: PipelineRun
  targetApiVersion: tekton.dev/v1beta1
  checkInterval: 300  # Check every 5 minutes

---
# Example PipelineRun with TTL
apiVersion: tekton.dev/v1beta1
kind: PipelineRun
metadata:
  name: example-pipeline

spec:
  ttlSecondsAfterFinished: 3600  # Delete 1 hour after completion
  pipelineSpec:
    # ... pipeline definition
```

### Custom Resource Cleanup

```yaml
apiVersion: ttlreaper.io/v1alpha1
kind: TTLReaperConfig
metadata:
  name: custom-job-cleanup
spec:
  targetKind: CustomJob
  targetApiVersion: example.com/v1
  ttlFieldPath: "spec.retentionPolicy.ttlSeconds"
  checkInterval: 600  # Check every 10 minutes
```

## Development

### Project Structure

```
ttl-reaper/
├── cmd/manager/          # Main application entry point
├── pkg/
│   ├── apis/
│   │   └── ttlreaper/
│   │       └── v1alpha1/ # API definitions
│   └── controller/       # Controller implementation
├── config/
│   ├── crd/              # Custom Resource Definitions
│   ├── rbac/             # RBAC manifests
│   └── deployment/       # Deployment manifests
├── examples/             # Example configurations
└── Makefile             # Build and deployment tasks
```

### Testing

```bash
# Run tests
make test

# Format code
make fmt

# Lint code
make vet

# Generate code (DeepCopy methods)
make generate

# Build with ko
make ko-build

# Deploy with ko
make ko-apply

# Local development targets
make kind-cluster        # Create kind cluster if needed
make kind-deploy         # Deploy controller to kind
make kind-delete         # Delete the kind cluster
```

### Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests for new functionality
5. Submit a pull request

## RBAC Requirements

The controller requires the following permissions:

- **TTLReaperConfig resources**: Full CRUD operations
- **Target custom resources**: Read and delete permissions
- **ConfigMaps/Leases**: For leader election

## Monitoring and Observability

The controller exposes:

- **Health checks**: `/healthz` and `/readyz` endpoints
- **Metrics**: Prometheus metrics on `:8080/metrics`

## Troubleshooting

### Common Issues

1. **Resources not being deleted**:
   - Verify the target resource has the correct TTL field
   - Check that the resource has completed (status conditions)
   - Ensure the TTL has actually expired

2. **Permission errors**:
   - Verify RBAC permissions for the target resource kind
   - Check that the service account has necessary cluster roles

3. **Configuration not working**:
   - Verify the `targetApiVersion` matches exactly
   - Check controller logs for error messages

### Debugging

```bash
# Check TTLReaperConfig
kubectl get ttlreaperconfigs

# Check controller logs
kubectl logs -n ttl-reaper-system deployment/ttl-reaper-controller

# List resources being monitored
kubectl get <target-kind> -n <target-namespace>
```

## License

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.