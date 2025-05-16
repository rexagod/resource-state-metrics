# Variables are declared in the order in which they occur.
ASSETS_DIR ?= assets
BENCH_TIMEOUT ?= 300
BOILERPLATE_GO_COMPLIANT ?= hack/boilerplate.go.txt
BOILERPLATE_YAML_COMPLIANT ?= hack/boilerplate.yaml.txt
BRANCH = $(shell git rev-parse --abbrev-ref HEAD)
BUILD_DATE ?= $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')
BUILD_TAG ?= $(shell git describe --tags --exact-match 2>/dev/null || echo "latest")
CODE_GENERATOR_VERSION ?= v0.31.2
COMMON = github.com/prometheus/common
CONTROLLER_GEN ?= $(shell which controller-gen)
CONTROLLER_GEN_APIS_DIR ?= pkg/apis
CONTROLLER_GEN_OUT_DIR ?= /tmp/resource-state-metrics/controller-gen
CONTROLLER_GEN_VERSION ?= v0.16.5
GIT_COMMIT = $(shell git rev-parse --short HEAD)
GO ?= go
GOLANGCI_LINT ?= $(shell which golangci-lint)
GOLANGCI_LINT_CONFIG ?= .golangci.yaml
GOLANGCI_LINT_VERSION ?= v1.62.0
GO_FILES = $(shell find . -type d -name vendor -prune -o -type f -name "*.go" -print)
KUBECTL ?= kubectl
KUBESTATEMETRICS_CUSTOMRESOURCESTATE_CONFIG ?= tests/bench/kubestatemetrics-custom-resource-state-config.yaml
LOCAL_NAMESPACE ?= default
MARKDOWNFMT ?= $(shell which markdownfmt)
MARKDOWNFMT_VERSION ?= v3.1.0
MD_FILES = $(shell find . \( -type d -name 'vendor' -o -type d -name $(patsubst %/,%,$(patsubst ./%,%,$(ASSETS_DIR))) \) -prune -o -type f -name "*.md" -print)
PPROF_OPTIONS ?=
PPROF_PORT ?= 9998
PROJECT_NAME = resource-state-metrics
RUNNER = $(shell id -u -n)@$(shell hostname)
TEST_PKG ?= ./tests
TEST_RUN_PATTERN ?= .
TEST_TIMEOUT ?= 240
V ?= 4
VALE ?= vale
VALE_ARCH ?= $(if $(filter $(shell uname -m),arm64),macOS_arm64,Linux_64-bit)
VALE_STYLES_DIR ?= /tmp/.vale/styles
VALE_VERSION ?= 3.1.0
VERSION = $(shell cat VERSION)

all: lint $(PROJECT_NAME)

#########
# Setup #
#########

.PHONY: setup
setup:
	# Setup vale.
	@if [ ! -f $(ASSETS_DIR)/$(VALE) ]; then \
    wget https://github.com/errata-ai/vale/releases/download/v$(VALE_VERSION)/vale_$(VALE_VERSION)_$(VALE_ARCH).tar.gz && \
    mkdir -p assets && tar -xvzf vale_$(VALE_VERSION)_$(VALE_ARCH).tar.gz -C $(ASSETS_DIR) && \
    rm vale_$(VALE_VERSION)_$(VALE_ARCH).tar.gz && \
    chmod +x $(ASSETS_DIR)/$(VALE); \
	fi
	# Setup markdownfmt.
	@$(GO) install github.com/Kunde21/markdownfmt/v3/cmd/markdownfmt@$(MARKDOWNFMT_VERSION)
	# Setup golangci-lint.
	@$(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
	# Setup controller-gen.
	@$(GO) install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_GEN_VERSION)
	# Setup code-generator.
	@$(GO) install k8s.io/code-generator/cmd/...@$(CODE_GENERATOR_VERSION)

##############
# Generating #
##############

.PHONY: manifests
manifests:
	@# Populate manifests/.
	@$(CONTROLLER_GEN) object:headerFile=$(BOILERPLATE_GO_COMPLIANT) \
	rbac:headerFile=$(BOILERPLATE_YAML_COMPLIANT),roleName=$(PROJECT_NAME) crd:headerFile=$(BOILERPLATE_YAML_COMPLIANT) paths=./$(CONTROLLER_GEN_APIS_DIR)/... \
	output:rbac:artifacts:config=$(CONTROLLER_GEN_OUT_DIR) output:crd:dir=$(CONTROLLER_GEN_OUT_DIR) && \
	mv "$(CONTROLLER_GEN_OUT_DIR)/resource-state-metrics.instrumentation.k8s-sigs.io_resourcemetricsmonitors.yaml" "manifests/custom-resource-definition.yaml" && \
	mv "$(CONTROLLER_GEN_OUT_DIR)/role.yaml" "manifests/cluster-role.yaml"

.PHONY: codegen
codegen:
	@# Populate pkg/generated/.
	@./hack/update-codegen.sh

.PHONY: verify_codegen
verify_codegen:
	@# Verify codegen.
	@./hack/verify-codegen.sh

.PHONY: generate
generate: manifests codegen

############
# Building #
############

.PHONY: image
image: $(PROJECT_NAME)
	@docker build -t $(PROJECT_NAME):$(BUILD_TAG) .

$(PROJECT_NAME): $(GO_FILES)
	@$(GO) build -a -installsuffix cgo -ldflags "-s -w \
	-X ${COMMON}/version.Version=v${VERSION} \
	-X ${COMMON}/version.Revision=${GIT_COMMIT} \
	-X ${COMMON}/version.Branch=${BRANCH} \
	-X ${COMMON}/version.BuildUser=${RUNNER} \
	-X ${COMMON}/version.BuildDate=${BUILD_DATE}" \
	-o $@

.PHONY: build
build: $(PROJECT_NAME)

###########
# Running #
###########

.PHONY: load
load: image
	@kind load docker-image $(PROJECT_NAME):$(BUILD_TAG)

.PHONY: apply
apply: manifests delete
	# Applying manifests/
	@$(KUBECTL) apply -f manifests/custom-resource-definition.yaml && \
	$(KUBECTL) apply -f manifests/
	# Applied manifests/

.PHONY: delete
delete:
	# Deleting manifests/
	@$(KUBECTL) delete -f manifests/ || true
	# Deleted manifests/

.PHONY: apply_testdata
apply_testdata: delete_testdata
	# Applying testdata/
	@$(KUBECTL) apply -f testdata/custom-resource-definition/ && \
	$(KUBECTL) apply -f testdata/custom-resource/
	# Applied testdata/

.PHONY: delete_testdata
delete_testdata:
	# Deleting testdata/
	@$(KUBECTL) delete -Rf testdata || true
	# Deleted testdata/

.PHONY: local
local: vet manifests codegen $(PROJECT_NAME)
	@$(KUBECTL) scale deployment $(PROJECT_NAME) --replicas=0 -n $(LOCAL_NAMESPACE) 2>/dev/null || true
	@./$(PROJECT_NAME) -v=$(V) -kubeconfig $(KUBECONFIG)

###########
# Testing #
###########

.PHONY: pprof
pprof:
	@go tool pprof ":$(PPROF_PORT)" $(PPROF_OPTIONS)

.PHONY: test_unit
test_unit:
	@$(GO) test $(shell go list ./... | \
		grep -v "/generated" | \
		grep -v "/signals" | \
		grep -v "/tests" | \
		grep -v "/version")

.PHONY: test_race
test_race:
	@$(GO) test -race $(shell go list ./... | \
		grep -v "/generated" | \
		grep -v "/signals" | \
		grep -v "/tests" | \
		grep -v "/version")

.PHONY: test_e2e
test_e2e:
	@\
	RSM_SELF_PORT=8887 \
	RSM_MAIN_PORT=8888 \
	GO=$(GO) \
	TEST_PKG=$(TEST_PKG) \
	TEST_RUN_PATTERN=$(TEST_RUN_PATTERN) \
	TEST_TIMEOUT=$(TEST_TIMEOUT) \
	timeout --signal SIGINT --preserve-status $(TEST_TIMEOUT) ./tests/run.sh

.PHONY: test
test: test_unit test_race test_e2e

.PHONY: bench
bench: vet setup manifests codegen build apply apply_testdata
	@\
	GO=$(GO) \
	KUBECONFIG=$(KUBECONFIG) \
	KUBECTL=$(KUBECTL) \
	KUBESTATEMETRICS_CUSTOMRESOURCESTATE_CONFIG=$(KUBESTATEMETRICS_CUSTOMRESOURCESTATE_CONFIG) \
	KUBESTATEMETRICS_DIR=$(KUBESTATEMETRICS_DIR) \
	LOCAL_NAMESPACE=$(LOCAL_NAMESPACE) \
	PROJECT_NAME=$(PROJECT_NAME) \
	timeout --preserve-status $(BENCH_TIMEOUT) ./tests/bench/bench.sh
	@make delete delete_testdata

###########
# Linting #
###########

.PHONY: vet
vet:
	@$(GO) vet ./...

.PHONY: clean
clean:
	@git clean -fxd

vale: .vale.ini $(MD_FILES)
	@mkdir -p $(VALE_STYLES_DIR) && \
	$(ASSETS_DIR)/$(VALE) sync && \
	$(ASSETS_DIR)/$(VALE) $(MD_FILES)

markdownfmt: $(MD_FILES)
	@test -z "$(shell $(MARKDOWNFMT) -l $(MD_FILES))" || (echo "\033[0;31mThe following files need to be formatted with 'markdownfmt -w -gofmt':" $(shell $(MARKDOWNFMT) -l $(MD_FILES)) "\033[0m" && exit 1)

markdownfmt-fix: $(MD_FILES)
	@for file in $(MD_FILES); do markdownfmt -w -gofmt $$file || exit 1; done

.PHONY: lint_md
lint_md: vale markdownfmt

.PHONY: lint_md_fix
lint_md_fix: vale markdownfmt-fix

golangci_lint: $(GO_FILES)
	@$(GOLANGCI_LINT) run -c $(GOLANGCI_LINT_CONFIG)

golangci_lint_fix: $(GO_FILES)
	@$(GOLANGCI_LINT) run --fix -c $(GOLANGCI_LINT_CONFIG)

.PHONY: lint-go
lint-go: golangci_lint

.PHONY: lint-go-fix
lint-go-fix: golangci_lint_fix

.PHONY: lint
lint: lint_md lint-go

.PHONY: lint-fix
lint-fix: lint_md_fix lint-go-fix
