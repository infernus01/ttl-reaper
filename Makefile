# Image URL to use all building/pushing image targets
IMG ?= ttl-reaper:latest
KO_DOCKER_REPO ?= ttl-reaper

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# Setting SHELL to bash allows bash commands to be executed by recipes.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

.PHONY: all
all: build

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk commands is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: test
test: fmt vet ## Run tests.
	go test ./... -coverprofile cover.out

##@ Build

.PHONY: build
build: fmt vet ## Build manager binary.
	go build -o bin/manager cmd/manager/main.go

.PHONY: run
run: fmt vet ## Run a controller from your host.
	go run cmd/manager/main.go

.PHONY: ko-build
ko-build: ## Build container image with ko.
	ko build --local ./cmd/manager

.PHONY: ko-push
ko-push: ## Push container image with ko.
	ko build ./cmd/manager

.PHONY: ko-apply
ko-apply: ## Deploy controller using ko.
	ko apply -f config/crd/
	ko apply -f config/rbac/
	ko apply -f config/deployment/

.PHONY: generate
generate: ## Generate code (deepcopy, etc.)
	@echo "Generated code is already present in zz_generated.deepcopy.go"

##@ Deployment

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: install
install: ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	kubectl apply -f config/crd/

.PHONY: uninstall
uninstall: ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config.
	kubectl delete -f config/crd/

.PHONY: deploy
deploy: ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	kubectl apply -f config/crd/
	kubectl apply -f config/rbac/
	kubectl apply -f config/deployment/

.PHONY: undeploy
undeploy: ## Undeploy controller from the K8s cluster specified in ~/.kube/config.
	kubectl delete -f config/deployment/ --ignore-not-found=$(ignore-not-found)
	kubectl delete -f config/rbac/ --ignore-not-found=$(ignore-not-found)

.PHONY: deploy-examples
deploy-examples: ## Deploy example configurations.
	kubectl apply -f examples/

.PHONY: clean
clean: ## Clean up build artifacts.
	rm -rf bin/
