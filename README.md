# TTL Reaper Controller

A Kubernetes controller that automatically deletes custom resources after their TTL (Time To Live) expires.

## How it works

This controller:
- Watches TTLReaper custom resources using generated clients
- For each TTLReaper, discovers and monitors the specified custom resource type
- Checks if resources have the `spec.ttlSecondsAfterFinished` field set
- Validates that resources are completed/finished using common completion patterns
- Calculates expiration time based on completion time + TTL duration
- Automatically deletes expired resources

## Supported Resource Patterns

The TTL reaper can work with any custom resource that:
1. Has a `spec.ttlSecondsAfterFinished` field (integer, seconds)
2. Indicates completion through one of these patterns:
   - `status.phase` = "Succeeded", "Failed", or "Completed"
   - `status.conditions` with type="Succeeded" and status="True"
   - `status.completionTime` field exists


## Example Configurations

### Monitor Tekton PipelineRuns

```yaml
apiVersion: clusterops.io/v1alpha1
kind: TTLReaper
metadata:
  name: tekton-pipelinerun-reaper
spec:
  targetKind: PipelineRun
  targetAPIVersion: tekton.dev/v1beta1
  # Optional: specific namespace
  targetNamespace: ci-cd
  # Optional: label selector
  labelSelector:
    matchLabels:
      app: ci-pipeline
```

### Monitor Kubernetes Jobs

```yaml
apiVersion: clusterops.io/v1alpha1
kind: TTLReaper
metadata:
  name: job-reaper
spec:
  targetKind: Job
  targetAPIVersion: batch/v1
  targetNamespace: batch-processing
```

### Monitor Custom Resources

```yaml
apiVersion: clusterops.io/v1alpha1
kind: TTLReaper
metadata:
  name: workflow-reaper
spec:
  targetKind: Workflow
  targetAPIVersion: workflows.example.com/v1
  # Empty targetNamespace means cluster-wide monitoring
```

## Container Deployment

The controller can be containerized and deployed using [ko](https://ko.build/):

```bash
# Build and push image
ko build github.com/infernus01/knative-demo/cmd/controller

# Deploy to cluster  
kubectl apply -f config/deploy/deployment.yaml
```

## Resource TTL Configuration

Any custom resource can be configured for automatic cleanup by adding the TTL field:

```yaml
apiVersion: tekton.dev/v1beta1
kind: PipelineRun
metadata:
  name: example-pipeline
  namespace: default
spec:
  # This PipelineRun will be deleted 300 seconds (5 minutes) after completion
  ttlSecondsAfterFinished: 300
  pipelineRef:
    name: my-pipeline
```

The TTL countdown starts when the resource reaches a completed state (succeeded, failed, etc.).

