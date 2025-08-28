# Basic Makefile for TTLReaper CRD

.PHONY: install-kind apply-crd apply-cr setup clean deploy-ttlreaper build-image deploy-ko

# Install kind cluster
install-kind:
	@echo "Creating kind cluster..."
	@kind create cluster --name ttlreaper-demo

# Apply ALL CRDs (any YAML file in config/crd/)
apply-crds:
	@echo "Applying all CRDs..."
	@if [ -d "config/crd" ]; then \
		kubectl apply -f config/crd/; \
		echo "Applied all CRDs from config/crd/"; \
	else \
		echo "config/crd/ directory not found"; \
	fi

# Apply ALL Custom Resources (any YAML file in examples/)
apply-crs:
	@echo "Applying all Custom Resources..."
	@if [ -d "examples" ]; then \
		kubectl apply -f examples/; \
		echo "Applied all CRs from examples/"; \
	fi

# Full setup: install kind + apply ALL CRDs + apply ALL CRs
setup: install-kind apply-crds apply-crs
	@echo "Setup complete!"

# Deploy using ko (alternative)
deploy-ttlreaper:
	@echo "Deploying ttlreaper controller using ko..."
	@KIND_CLUSTER_NAME=ttlreaper-demo KO_DOCKER_REPO=kind.local ko apply -f config/deploy/

# Clean up everything (cluster)
clean:
	@echo "Cleaning up..."
	@kubectl delete ttlreapers --all || echo "No TTLReaper resources to delete"
	@kubectl delete namespace ttlreaper-system --ignore-not-found=true
	@kind get clusters | grep -q ttlreaper-demo && kind delete cluster --name ttlreaper-demo || echo "Cluster 'ttlreaper-demo' not found"