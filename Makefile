SHELL := /bin/bash

MONGODB_COMMUNITY_CONFIG ?= $(HOME)/.community-operator-dev/config.json

# Image URL to use all building/pushing image targets
REPO_URL := $(shell jq -r .repo_url < $(MONGODB_COMMUNITY_CONFIG))
OPERATOR_IMAGE := $(shell jq -r .operator_image < $(MONGODB_COMMUNITY_CONFIG))
NAMESPACE := $(shell jq -r .namespace < $(MONGODB_COMMUNITY_CONFIG))
UPGRADE_HOOK_IMG := $(shell jq -r .version_upgrade_hook_image < $(MONGODB_COMMUNITY_CONFIG))
READINESS_PROBE_IMG := $(shell jq -r .readiness_probe_image < $(MONGODB_COMMUNITY_CONFIG))
REGISTRY := $(shell jq -r .repo_url < $(MONGODB_COMMUNITY_CONFIG))
AGENT_IMAGE_NAME := $(shell jq -r .agent_image < $(MONGODB_COMMUNITY_CONFIG))
HELM_CHART ?= ./helm-charts/charts/community-operator

STRING_SET_VALUES := --set namespace=$(NAMESPACE),versionUpgradeHook.name=$(UPGRADE_HOOK_IMG),readinessProbe.name=$(READINESS_PROBE_IMG),registry.operator=$(REPO_URL),operator.operatorImageName=$(OPERATOR_IMAGE),operator.version=latest,registry.agent=$(REGISTRY),registry.versionUpgradeHook=$(REGISTRY),registry.readinessProbe=$(REGISTRY),registry.operator=$(REGISTRY),versionUpgradeHook.version=latest,readinessProbe.version=latest,agent.version=latest,agent.name=$(AGENT_IMAGE_NAME)
STRING_SET_VALUES_LOCAL := $(STRING_SET_VALUES) --set operator.replicas=0

DOCKERFILE ?= operator
# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:crdVersions=v1"
RELEASE_NAME_HELM ?= mongodb-kubernetes-operator
TEST_NAMESPACE ?= $(NAMESPACE)

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

BASE_GO_PACKAGE = github.com/mongodb/mongodb-kubernetes-operator
GO_LICENSES = go-licenses
DISALLOWED_LICENSES = restricted # found reciprocal MPL-2.0

all: manager

##@ Development

fmt: ## Run go fmt against code
	go fmt ./...

vet: ## Run go vet against code
	go vet ./...

generate: controller-gen ## Generate code
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

$(GO_LICENSES):
	@if ! which $@ &> /dev/null; then \
	go install github.com/google/go-licenses@latest; \
	fi

licenses.csv: go.mod $(GO_LICENSES) ## Track licenses in a CSV file
	@echo "Tracking licenses into file $@"
	@echo "========================================"
	GOOS=linux GOARCH=amd64 $(GO_LICENSES) csv --include_tests $(BASE_GO_PACKAGE)/... > $@

# We only check that go.mod is NOT newer than licenses.csv because the CI
# tends to generate slightly different results, so content comparison wouldn't work
licenses-tracked: ## Checks license.csv is up to date
	@if [ go.mod -nt licenses.csv ]; then \
	echo "License.csv is stale! Please run 'make licenses.csv' and commit"; exit 1; \
	else echo "License.csv OK (up to date)"; fi

.PHONY: check-licenses-compliance
check-licenses-compliance: licenses.csv  ## Check licenses are compliant with our restrictions
	@echo "Checking licenses not to be: $(DISALLOWED_LICENSES)"
	@echo "============================================"
	GOOS=linux GOARCH=amd64 $(GO_LICENSES) check --include_tests $(BASE_GO_PACKAGE)/... \
	--disallowed_types $(DISALLOWED_LICENSES)
	@echo "--------------------"
	@echo "Licenses check: PASS"

.PHONY: check-licenses
check-licenses: licenses-tracked check-licenses-compliance ## Check license tracking & compliance

TEST ?= ./pkg/... ./api/... ./cmd/... ./controllers/... ./test/e2e/util/mongotester/...
test: generate fmt vet manifests ## Run unit tests
	go test $(options) $(TEST) -coverprofile cover.out

manager: generate fmt vet ## Build operator binary
	go build -o bin/manager ./cmd/manager/main.go

run: install ## Run the operator against the configured Kubernetes cluster in ~/.kube/config
	eval $$(scripts/dev/get_e2e_env_vars.py $(cleanup)); \
	go run ./cmd/manager/main.go

debug: install install-rbac ## Run the operator in debug mode with dlv
	eval $$(scripts/dev/get_e2e_env_vars.py $(cleanup)); \
	dlv debug ./cmd/manager/main.go

CONTROLLER_GEN = $(shell pwd)/bin/controller-gen
controller-gen: ## Download controller-gen locally if necessary
	$(call go-install-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen@v0.15.0)

# Try to use already installed helm from PATH
ifeq (ok,$(shell test -f "$$(which helm)" && echo ok))
    HELM=$(shell which helm)
else
    HELM=/usr/local/bin/helm
endif

helm: ## Download helm locally if necessary
	$(call install-helm)

install-prerequisites-macos: ## installs prerequisites for macos development
	scripts/dev/install_prerequisites.sh

##@ Installation/Uninstallation

install: manifests helm install-crd ## Install CRDs into a cluster

install-crd:
	kubectl apply -f config/crd/bases/mongodbcommunity.mongodb.com_mongodbcommunity.yaml

install-chart: uninstall-crd
	$(HELM) upgrade --install $(STRING_SET_VALUES) $(RELEASE_NAME_HELM) $(HELM_CHART) --namespace $(NAMESPACE) --create-namespace

install-chart-local-operator: uninstall-crd
	$(HELM) upgrade --install $(STRING_SET_VALUES_LOCAL) $(RELEASE_NAME_HELM) $(HELM_CHART) --namespace $(NAMESPACE) --create-namespace

prepare-local-dev: generate-env-file install-chart-local-operator install-rbac setup-sas

# patches all sas to use the local-image-registry
setup-sas:
	scripts/dev/setup_sa.sh

install-chart-with-tls-enabled:
	$(HELM) upgrade --install --set createResource=true $(STRING_SET_VALUES) $(RELEASE_NAME_HELM) $(HELM_CHART) --namespace $(NAMESPACE) --create-namespace

install-rbac:
	$(HELM) template $(STRING_SET_VALUES) -s templates/database_roles.yaml $(HELM_CHART) | kubectl apply -f -
	$(HELM) template $(STRING_SET_VALUES) -s templates/operator_roles.yaml $(HELM_CHART) | kubectl apply -f -

uninstall-crd:
	kubectl delete crd --ignore-not-found mongodbcommunity.mongodbcommunity.mongodb.com

uninstall-chart:
	$(HELM) uninstall $(RELEASE_NAME_HELM) -n $(NAMESPACE)

uninstall-rbac:
	$(HELM) template $(STRING_SET_VALUES) -s templates/database_roles.yaml $(HELM_CHART) | kubectl delete -f -
	$(HELM) template $(STRING_SET_VALUES) -s templates/operator_roles.yaml $(HELM_CHART) | kubectl delete -f -

uninstall: manifests helm uninstall-chart uninstall-crd ## Uninstall CRDs from a cluster

##@ Deployment

deploy: manifests helm install-chart install-crd ## Deploy controller in the configured Kubernetes cluster in ~/.kube/config

undeploy: uninstall-chart uninstall-crd ## UnDeploy controller from the configured Kubernetes cluster in ~/.kube/config

manifests: controller-gen ## Generate manifests e.g. CRD, RBAC etc.
	$(CONTROLLER_GEN) $(CRD_OPTIONS) paths="./..." output:crd:artifacts:config=config/crd/bases
	cp config/crd/bases/* $(HELM_CHART)/crds

##@ E2E

# Run e2e tests locally using go build while also setting up a proxy in the shell to allow
# the test to run as if it were inside the cluster. This enables mongodb connectivity while running locally.
# "MDB_LOCAL_OPERATOR=true" ensures the operator pod is not spun up while running the e2e test - since you're
# running it locally.
e2e-telepresence: cleanup-e2e install ## Run e2e tests locally using go build while also setting up a proxy e.g. make e2e-telepresence test=replica_set cleanup=true
	export MDB_LOCAL_OPERATOR=true; \
	telepresence connect; \
	eval $$(scripts/dev/get_e2e_env_vars.py $(cleanup)); \
	go test -v -timeout=30m -failfast $(options) ./test/e2e/$(test) ; \
	telepresence quit

e2e-k8s: cleanup-e2e install e2e-image ## Run e2e test by deploying test image in kubernetes, you can provide e2e.py flags e.g. make e2e-k8s test=replica_set e2eflags="--perform-cleanup".
	python scripts/dev/e2e.py $(e2eflags) --test $(test)

e2e: cleanup-e2e install ## Run e2e test locally. e.g. make e2e test=replica_set cleanup=true
	eval $$(scripts/dev/get_e2e_env_vars.py $(cleanup)); \
	go test -v -short -timeout=30m -failfast $(options) ./test/e2e/$(test)

e2e-gh: ## Trigger a Github Action of the given test
	scripts/dev/run_e2e_gh.sh $(test)

cleanup-e2e: ## Cleans up e2e test env
	kubectl delete mdbc,all,secrets -l e2e-test=true -n ${TEST_NAMESPACE} || true
	# Most of the tests use StatefulSets, which in turn use stable storage. In order to
	# avoid interleaving tests with each other, we need to drop them all.
	kubectl delete pvc --all -n $(NAMESPACE) || true
	kubectl delete pv --all -n $(NAMESPACE) || true

generate-env-file: ## generates a local-test.env for local testing
	mkdir -p .community-operator-dev
	{ python scripts/dev/get_e2e_env_vars.py | tee >(cut -d' ' -f2 > .community-operator-dev/local-test.env) ;} > .community-operator-dev/local-test.export.env
	. .community-operator-dev/local-test.export.env

##@ Image

operator-image: ## Build and push the operator image
	python pipeline.py --image-name operator $(IMG_BUILD_ARGS)

e2e-image: ## Build and push e2e test image
	python pipeline.py --image-name e2e $(IMG_BUILD_ARGS)

agent-image: ## Build and push agent image
	python pipeline.py --image-name agent $(IMG_BUILD_ARGS)

readiness-probe-image: ## Build and push readiness probe image
	python pipeline.py --image-name readiness-probe $(IMG_BUILD_ARGS)

version-upgrade-post-start-hook-image: ## Build and push version upgrade post start hook image
	python pipeline.py --image-name version-upgrade-hook $(IMG_BUILD_ARGS)

all-images: operator-image e2e-image agent-image readiness-probe-image version-upgrade-post-start-hook-image ## create all required images

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

help: ## Show this help screen.
	@echo 'Usage: make <OPTIONS> ... <TARGETS>'
	@echo ''
	@echo 'Available targets are:'
	@echo ''
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z0-9_-]+:.*?##/ { printf "  \033[36m%-25s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)
