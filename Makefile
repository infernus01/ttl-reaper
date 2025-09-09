# Basic Makefile for TTLReaper CRD

.PHONY: install-kind setup clean deploy-ttlreaper

# Install kind cluster
install-kind:
	@echo "Creating kind cluster..."
	@kind create cluster --name ttlreaper-demo

# Deploy using ko (alternative)
deploy-ttlreaper:
	@echo "Deploying ttlreaper controller using ko..."
	@KIND_CLUSTER_NAME=ttlreaper-demo KO_DOCKER_REPO=kind.local ko apply -Rf config/

# Full setup: install kind + apply ALL CRDs
setup: install-kind deploy-ttlreaper
	@echo "Setup complete!"
	
# Clean up everything (cluster)
clean:
	@echo "Cleaning up..."
	@kubectl delete ttlreapers --all || echo "No TTLReaper resources to delete"
	@kubectl delete namespace ttlreaper-system --ignore-not-found=true
	@kind get clusters | grep -q ttlreaper-demo && kind delete cluster --name ttlreaper-demo || echo "Cluster 'ttlreaper-demo' not found"
