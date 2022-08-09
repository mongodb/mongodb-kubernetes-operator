SHELL := /bin/bash

MONGODB_COMMUNITY_CONFIG ?= $(HOME)/.community-operator-dev/config.json

# Image URL to use all building/pushing image targets
REPO_URL := $(shell jq -r .repo_url < $(MONGODB_COMMUNITY_CONFIG))
OPERATOR_IMAGE := $(shell jq -r .operator_image < $(MONGODB_COMMUNITY_CONFIG))
NAMESPACE := $(shell jq -r .namespace < $(MONGODB_COMMUNITY_CONFIG))
UPGRADE_HOOK_IMG := $(shell jq -r .version_upgrade_hook_image < $(MONGODB_COMMUNITY_CONFIG))
READINESS_PROBE_IMG := $(shell jq -r .readiness_probe_image < $(MONGODB_COMMUNITY_CONFIG))
REGISTRY := $(shell jq -r .repo_url < $(MONGODB_COMMUNITY_CONFIG))
AGENT_IMAGE_NAME := $(shell jq -r .agent_image_ubuntu < $(MONGODB_COMMUNITY_CONFIG))

HELM_CHART ?= ./helm-charts/charts/community-operator

STRING_SET_VALUES := --set namespace=$(NAMESPACE),versionUpgradeHook.name=$(UPGRADE_HOOK_IMG),readinessProbe.name=$(READINESS_PROBE_IMG),registry.operator=$(REPO_URL),operator.operatorImageName=$(OPERATOR_IMAGE),operator.version=latest,registry.agent=$(REGISTRY),registry.versionUpgradeHook=$(REGISTRY),registry.readinessProbe=$(REGISTRY),registry.operator=$(REGISTRY),versionUpgradeHook.version=latest,readinessProbe.version=latest,agent.version=latest,agent.name=$(AGENT_IMAGE_NAME)

DOCKERFILE ?= operator
# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true,crdVersions=v1"
RELEASE_NAME_HELM ?= mongodb-kubernetes-operator
TEST_NAMESPACE ?= $(NAMESPACE)

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

all: manager

# Run unit tests
TEST ?= ./pkg/... ./api/... ./cmd/... ./controllers/... ./test/e2e/util/mongotester/...
test: generate fmt vet manifests
	go test $(TEST) -coverprofile cover.out

# Build manager binary
manager: generate fmt vet
	go build -o bin/manager ./cmd/manager/main.go

# Run against the configured Kubernetes cluster in ~/.kube/config
run: install install-rbac
	eval $$(scripts/dev/get_e2e_env_vars.py $(cleanup)); \
	go run ./cmd/manager/main.go

debug: install install-rbac
	eval $$(scripts/dev/get_e2e_env_vars.py $(cleanup)); \
	dlv debug ./cmd/manager/main.go

# Install CRDs into a cluster
install: manifests helm install-crd

install-crd:
	kubectl apply -f config/crd/bases/mongodbcommunity.mongodb.com_mongodbcommunity.yaml

install-chart:
	$(HELM) upgrade --install $(STRING_SET_VALUES) $(RELEASE_NAME_HELM) $(HELM_CHART)

install-chart-with-tls-enabled:
	$(HELM) upgrade --install --set createResource=true $(STRING_SET_VALUES) $(RELEASE_NAME_HELM) $(HELM_CHART)

install-rbac:
	$(HELM) template $(STRING_SET_VALUES) -s templates/database_roles.yaml $(HELM_CHART) | kubectl apply -f -
	$(HELM) template $(STRING_SET_VALUES) -s templates/operator_roles.yaml $(HELM_CHART) | kubectl apply -f -

uninstall-crd:
	kubectl delete crd mongodbcommunity.mongodbcommunity.mongodb.com

uninstall-chart:
	$(HELM) uninstall $(RELEASE_NAME_HELM) -n $(NAMESPACE)

uninstall-rbac:
	$(HELM) template $(STRING_SET_VALUES) -s templates/database_roles.yaml $(HELM_CHART) | kubectl delete -f -
	$(HELM) template $(STRING_SET_VALUES) -s templates/operator_roles.yaml $(HELM_CHART) | kubectl delete -f -


# Uninstall CRDs from a cluster
uninstall: manifests helm uninstall-chart uninstall-crd

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: manifests helm install-chart install-crd

# UnDeploy controller from the configured Kubernetes cluster in ~/.kube/config
undeploy: uninstall-chart uninstall-crd

# Generate manifests e.g. CRD, RBAC etc.
manifests: controller-gen
	$(CONTROLLER_GEN) $(CRD_OPTIONS) paths="./..." output:crd:artifacts:config=config/crd/bases
	cp config/crd/bases/* $(HELM_CHART)/crds

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet ./...

# Run e2e tests locally using go build while also setting up a proxy in the shell to allow
# the test to run as if it were inside the cluster. This enables mongodb connectivity while running locally.
e2e-telepresence: cleanup-e2e install
	telepresence connect; \
	telepresence status; \
	eval $$(scripts/dev/get_e2e_env_vars.py $(cleanup)); \
	go test -v -timeout=30m -failfast ./test/e2e/$(test); \
	telepresence quit

# Run e2e test by deploying test image in kubernetes.
e2e-k8s: cleanup-e2e install e2e-image
	python scripts/dev/e2e.py --perform-cleanup --test $(test)

# Run e2e test locally.
# e.g. make e2e test=replica_set cleanup=true
e2e: cleanup-e2e install
	eval $$(scripts/dev/get_e2e_env_vars.py $(cleanup)); \
	go test -v -short -timeout=30m -failfast ./test/e2e/$(test)

# Trigger a Github Action of the given test
e2e-gh:
	scripts/dev/run_e2e_gh.sh $(test)

cleanup-e2e:
	kubectl delete mdbc,all,secrets -l e2e-test=true -n ${TEST_NAMESPACE} || true
	# Most of the tests use StatefulSets, which in turn use stable storage. In order to
	# avoid interleaving tests with each other, we need to drop them all.
	kubectl delete pvc --all -n $(NAMESPACE) || true

# Generate code
generate: controller-gen
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

# Build and push the operator image
operator-image:
	python pipeline.py --image-name operator-ubi

# Build and push e2e test image
e2e-image:
	python pipeline.py --image-name e2e

# Build and push agent image
agent-image:
	python pipeline.py --image-name agent-ubuntu

# Build and push readiness probe image
readiness-probe-image:
	python pipeline.py --image-name readiness-probe-init

# Build and push version upgrade post start hook image
version-upgrade-post-start-hook-image:
	python pipeline.py --image-name version-post-start-hook-init

# create all required images
all-images: operator-image e2e-image agent-image readiness-probe-image version-upgrade-post-start-hook-image


# Download controller-gen locally if necessary
CONTROLLER_GEN = $(shell pwd)/bin/controller-gen
controller-gen:
	$(call go-install-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen@v0.4.1)

# Try to use already installed helm from PATH
ifeq (ok,$(shell test -f "$$(which helm)" && echo ok))
    HELM=$(shell which helm)
else
    HELM=/usr/local/bin/helm
endif

# Download helm locally if necessary
helm:
	$(call install-helm)

define install-helm
@[ -f $(HELM) ] || { \
set -e ;\
TMP_DIR=$$(mktemp -d) ;\
cd $$TMP_DIR ;\
curl -fsSL -o get_helm.sh https://raw.githubusercontent.com/helm/helm/master/scripts/get-helm-3 ;\
chmod 700 get_helm.sh ;\
./get_helm.sh ;\
rm -rf $(TMP_DIR) ;\
}
endef

# go-install-tool will 'go install' any package $2 and install it to $1.
PROJECT_DIR := $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))
define go-install-tool
@[ -f $(1) ] || { \
set -e ;\
TMP_DIR=$$(mktemp -d) ;\
cd $$TMP_DIR ;\
go mod init tmp ;\
echo "Downloading $(2)" ;\
GOBIN=$(PROJECT_DIR)/bin go install $(2) ;\
rm -rf $$TMP_DIR ;\
}
endef

install-prerequisites-macos:
	scripts/dev/install_prerequisites.sh
