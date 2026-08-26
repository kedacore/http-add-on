##################################################
# Variables                                      #
##################################################
SHELL          = /bin/bash
.SHELLFLAGS    = -eo pipefail -c # fail on error and pipeline failures
.DEFAULT_GOAL := ko-build

IMAGE_REGISTRY ?= ghcr.io
IMAGE_REPO     ?= kedacore
# exported so ko can use it in .ko.yaml ldflags
export VERSION ?= HEAD

IMAGE_OPERATOR     ?= $(IMAGE_REGISTRY)/$(IMAGE_REPO)/http-add-on-operator
IMAGE_INTERCEPTOR  ?= $(IMAGE_REGISTRY)/$(IMAGE_REPO)/http-add-on-interceptor
IMAGE_SCALER       ?= $(IMAGE_REGISTRY)/$(IMAGE_REPO)/http-add-on-scaler

GIT_COMMIT       ?= $(shell git rev-list -1 HEAD)
GIT_COMMIT_SHORT ?= $(shell git rev-parse --short HEAD)
DATE             ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

IMAGE_OPERATOR_VERSIONED_TAG     ?= $(IMAGE_OPERATOR):$(VERSION)
IMAGE_INTERCEPTOR_VERSIONED_TAG  ?= $(IMAGE_INTERCEPTOR):$(VERSION)
IMAGE_SCALER_VERSIONED_TAG       ?= $(IMAGE_SCALER):$(VERSION)

IMAGE_OPERATOR_SHA_TAG     ?= $(IMAGE_OPERATOR):$(GIT_COMMIT_SHORT)
IMAGE_INTERCEPTOR_SHA_TAG  ?= $(IMAGE_INTERCEPTOR):$(GIT_COMMIT_SHORT)
IMAGE_SCALER_SHA_TAG       ?= $(IMAGE_SCALER):$(GIT_COMMIT_SHORT)

KO_RELEASE_PLATFORMS ?= linux/amd64,linux/arm64,linux/s390x

# renovate: datasource=helm depName=cert-manager registryUrl=https://charts.jetstack.io
CERT_MANAGER_VERSION ?= v1.21.1
# renovate: datasource=helm depName=jaeger registryUrl=https://jaegertracing.github.io/helm-charts
JAEGER_VERSION ?= 4.12.0
# renovate: datasource=helm depName=keda registryUrl=https://kedacore.github.io/charts
KEDA_VERSION ?= 2.20.2
# renovate: datasource=helm depName=opentelemetry-collector registryUrl=https://open-telemetry.github.io/opentelemetry-helm-charts
OTEL_COLLECTOR_VERSION ?= 0.170.0

HELM_RETRIES ?= 3
HELM_RETRY_DELAY ?= 30

define helm-retry
	for ((i=1; i<=$(HELM_RETRIES); i++)); do \
		$(1) && break || { \
			if [ $$i -eq $(HELM_RETRIES) ]; then echo "ERROR: helm command failed after $(HELM_RETRIES) attempts"; exit 1; fi; \
			echo "WARNING: helm command failed (attempt $$i/$(HELM_RETRIES)), retrying in $(HELM_RETRY_DELAY)s..."; \
			sleep $(HELM_RETRY_DELAY); \
		}; \
	done
endef

COSIGN_FLAGS ?= -y -a GIT_HASH=$(GIT_COMMIT) -a GIT_VERSION=$(VERSION) -a BUILD_DATE=$(DATE)

## Tool Binaries
CONTROLLER_GEN ?= go tool controller-gen

LOCALBIN ?= $(CURDIR)/bin

# renovate: datasource=github-releases depName=golangci/golangci-lint
GOLANGCI_LINT_VERSION ?= v2.13.1
# renovate: datasource=github-releases depName=kubernetes-sigs/kustomize extractVersion=^kustomize\/(?<version>.+)$$
KUSTOMIZE_VERSION ?= v5.8.1
# renovate: datasource=github-releases depName=helm/helm
HELM_VERSION ?= v4.2.4

GOLANGCI_LINT ?= $(LOCALBIN)/golangci-lint-$(GOLANGCI_LINT_VERSION)
KUSTOMIZE     ?= $(LOCALBIN)/kustomize-$(KUSTOMIZE_VERSION)
HELM          ?= $(LOCALBIN)/helm-$(HELM_VERSION)

##################################################
# Go build                                       #
##################################################

build-operator:
	go build -o bin/operator ./operator

build-interceptor:
	go build -o bin/interceptor ./interceptor

build-scaler:
	go build -o bin/scaler ./scaler

build: build-operator build-interceptor build-scaler

##################################################
# Ko build                                       #
##################################################

ko-build-operator:
	ko build --local ./operator

ko-build-interceptor:
	ko build --local ./interceptor

ko-build-scaler:
	ko build --local ./scaler

ko-build: ko-build-operator ko-build-interceptor ko-build-scaler

##################################################
# Testing                                        #
##################################################

.PHONY: test
test:
	go test ./...

FUZZ_TIME ?= 30s
FUZZ_TARGETS = \
	FuzzTableMemoryRoute:./pkg/routing/ \
	FuzzParseTLSVersion:./pkg/tls/ \
	FuzzParseCipherSuites:./pkg/tls/ \
	FuzzParseCurvePreferences:./pkg/tls/ \
	FuzzProxyHandler:./interceptor/ \
	FuzzEscapeString:./scaler/

.PHONY: fuzz
fuzz: ## Run all fuzz tests (FUZZ_TIME=30s by default)
	@for entry in $(FUZZ_TARGETS); do \
		func=$${entry%%:*}; pkg=$${entry#*:}; \
		echo "=== Fuzzing $$func in $$pkg ==="; \
		go test $$pkg -run='^$$' -fuzz=$$func -fuzztime=$(FUZZ_TIME) || exit 1; \
	done

e2e-test-legacy:
	go run -tags e2e ./tests/run-all.go

e2e-test-legacy-setup:
	ONLY_SETUP=true go run -tags e2e ./tests/run-all.go

e2e-test-legacy-local:
	SKIP_SETUP=true go run -tags e2e ./tests/run-all.go

E2E_PACKAGE = $(if $(PROFILE),./test/e2e/$(PROFILE)/...,./test/e2e/...)
e2e-test: ## Run e2e tests (PROFILE=tls, RUN=TestColdStart, E2E_ARGS="--labels=area=scaling --dry-run")
# -p 1 is needed to run only one profile (=addon configuration) in parallel
# -parallel 4 limits concurrent tests to avoid overwhelming the kubelet port-forward
	go test -tags e2e $(E2E_PACKAGE) -p 1 -count=1 -timeout 15m -v -parallel 4 $(if $(RUN),-run '$(RUN)') $(if $(E2E_ARGS),-args $(E2E_ARGS))

e2e-test-default: PROFILE = default
e2e-test-default: e2e-test

e2e-test-ci: ## Run all e2e tests (CI mode with retries)
# -p 1 is needed to run only one profile (=addon configuration) in parallel
# -parallel 4 limits concurrent tests to avoid overwhelming the kubelet port-forward
	go tool gotestsum --rerun-fails=2 --format=github-actions --packages="./test/e2e/..." -- -tags e2e -p 1 -count=1 -timeout 30m -v -parallel 4

e2e-test-images: ## Build all test images under test/images/ and push to $KO_DOCKER_REPO
	# --base-import-paths prevents the hash suffix in image names
	ko build --base-import-paths ./test/images/*/

e2e-deps-external: e2e-deps-cert-manager e2e-deps-jaeger e2e-deps-otel-collector ## Install non-KEDA e2e deps

e2e-deps: e2e-deps-external e2e-deps-keda ## Install all e2e dependencies

e2e-deps-cert-manager: $(HELM)
	$(HELM) repo add jetstack https://charts.jetstack.io --force-update
	$(call helm-retry,$(HELM) upgrade --install cert-manager jetstack/cert-manager \
		--namespace cert-manager --create-namespace \
		-f test/fixtures/cert-manager-values.yaml \
		--version $(CERT_MANAGER_VERSION) --wait --timeout 5m)

e2e-deps-jaeger: $(HELM)
	$(HELM) repo add jaegertracing https://jaegertracing.github.io/helm-charts --force-update
	$(call helm-retry,$(HELM) upgrade --install jaeger jaegertracing/jaeger \
		--namespace jaeger --create-namespace \
		-f test/fixtures/jaeger-values.yaml \
		--version $(JAEGER_VERSION) --wait --timeout 5m)

e2e-deps-keda: $(HELM)
	$(HELM) repo add kedacore https://kedacore.github.io/charts --force-update
	$(call helm-retry,$(HELM) upgrade --install keda kedacore/keda \
		--namespace keda --create-namespace \
		--version $(KEDA_VERSION) --wait --timeout 5m)

e2e-deps-otel-collector: $(HELM)
	$(HELM) repo add open-telemetry https://open-telemetry.github.io/opentelemetry-helm-charts --force-update
	$(call helm-retry,$(HELM) upgrade --install opentelemetry-collector open-telemetry/opentelemetry-collector \
		--namespace open-telemetry-system --create-namespace \
		-f test/fixtures/otel-values.yaml \
		--version $(OTEL_COLLECTOR_VERSION) --wait --timeout 5m)

e2e-setup: e2e-deps deploy e2e-test-images ## Full e2e setup: install deps + deploy http-add-on + build test images

benchmark-test: ## Run benchmark tests (BENCH_THROUGHPUT_RATE=300, BENCH_P99_MAX=2s, ...)
	BENCHMARK=true go test -tags e2e ./test/e2e/benchmark/... -p 1 -count=1 -timeout 30m -v $(if $(RUN),-run '$(RUN)')

benchmark-test-ci: ## Run benchmark tests (CI mode with retries)
	BENCHMARK=true go tool gotestsum --rerun-fails=1 --format=github-actions --packages="./test/e2e/benchmark/..." -- -tags e2e -p 1 -count=1 -timeout 30m -v

##################################################
# Code generation & manifests                    #
##################################################

generate: codegen manifests  ## Generate code and manifests.

generate-proto: ## Generate protobuf and gRPC stubs only used in e2e test images
	buf generate

codegen: ## Generate DeepCopy method implementations.
	$(CONTROLLER_GEN) object:headerFile='hack/boilerplate.go.txt' paths='./...'

manifests: ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) crd rbac:roleName='operator' webhook paths='./operator/...' output:crd:artifacts:config='config/crd/bases' output:rbac:artifacts:config='config/operator'
	$(CONTROLLER_GEN) crd rbac:roleName='scaler' webhook paths='./scaler/...' output:rbac:artifacts:config='config/scaler'
	$(CONTROLLER_GEN) crd rbac:roleName='interceptor' webhook paths='./interceptor/...' output:rbac:artifacts:config='config/interceptor'

verify-manifests: ## Verify manifests are up to date.
	./hack/verify-manifests.sh

##################################################
# Linting & static checks                        #
##################################################

fmt: $(GOLANGCI_LINT)
	$(GOLANGCI_LINT) fmt

lint: $(GOLANGCI_LINT)
	$(GOLANGCI_LINT) run

lint-fix: $(GOLANGCI_LINT)
	$(GOLANGCI_LINT) run --fix

check-links:
	lychee "./**/*.md"

pre-commit: ## Run static-checks.
	pre-commit run --all-files

##################################################
# Deployment (local cluster)                     #
##################################################

install: $(KUSTOMIZE)
	$(KUSTOMIZE) build config/crd | kubectl apply -f -

deploy: $(KUSTOMIZE)
	$(KUSTOMIZE) build config/default | ko apply -f -

deploy-operator: $(KUSTOMIZE)
	$(KUSTOMIZE) build config/operator | ko apply -f -

deploy-interceptor: $(KUSTOMIZE)
	$(KUSTOMIZE) build config/interceptor | ko apply -f -

deploy-scaler: $(KUSTOMIZE)
	$(KUSTOMIZE) build config/scaler | ko apply -f -

undeploy: $(KUSTOMIZE)
	$(KUSTOMIZE) build config/default | ko delete -f - || true

##################################################
# Publish, release & signing                     #
##################################################

publish-operator:
	# --bare preserves image names like ghcr.io/kedacore/http-add-on-operator
	KO_DOCKER_REPO=$(IMAGE_OPERATOR) ko build --bare --platform=$(KO_RELEASE_PLATFORMS) --tags=$(VERSION),$(GIT_COMMIT_SHORT) ./operator | tee operator.digest

publish-interceptor:
	KO_DOCKER_REPO=$(IMAGE_INTERCEPTOR) ko build --bare --platform=$(KO_RELEASE_PLATFORMS) --tags=$(VERSION),$(GIT_COMMIT_SHORT) ./interceptor | tee interceptor.digest

publish-scaler:
	KO_DOCKER_REPO=$(IMAGE_SCALER) ko build --bare --platform=$(KO_RELEASE_PLATFORMS) --tags=$(VERSION),$(GIT_COMMIT_SHORT) ./scaler | tee scaler.digest

publish: publish-operator publish-interceptor publish-scaler

release: manifests $(KUSTOMIZE) ## Produce new KEDA Http Add-on release in keda-add-ons-http-$(VERSION).yaml file.
	$(KUSTOMIZE) build config/crd > keda-add-ons-http-$(VERSION).yaml
	echo '---' >> keda-add-ons-http-$(VERSION).yaml
	$(KUSTOMIZE) build config/operator | KO_DOCKER_REPO=$(IMAGE_OPERATOR) ko resolve --bare --platform=$(KO_RELEASE_PLATFORMS) --tags=$(VERSION) -f - >> keda-add-ons-http-$(VERSION).yaml
	echo '---' >> keda-add-ons-http-$(VERSION).yaml
	$(KUSTOMIZE) build config/interceptor | KO_DOCKER_REPO=$(IMAGE_INTERCEPTOR) ko resolve --bare --platform=$(KO_RELEASE_PLATFORMS) --tags=$(VERSION) -f - >> keda-add-ons-http-$(VERSION).yaml
	echo '---' >> keda-add-ons-http-$(VERSION).yaml
	$(KUSTOMIZE) build config/scaler | KO_DOCKER_REPO=$(IMAGE_SCALER) ko resolve --bare --platform=$(KO_RELEASE_PLATFORMS) --tags=$(VERSION) -f - >> keda-add-ons-http-$(VERSION).yaml
	$(KUSTOMIZE) build config/crd > keda-add-ons-http-$(VERSION)-crds.yaml

sign-images: ## Sign KEDA images published on GitHub Container Registry
	cosign sign $(COSIGN_FLAGS) $(IMAGE_OPERATOR_VERSIONED_TAG)
	cosign sign $(COSIGN_FLAGS) $(IMAGE_OPERATOR_SHA_TAG)
	cosign sign $(COSIGN_FLAGS) $(IMAGE_INTERCEPTOR_VERSIONED_TAG)
	cosign sign $(COSIGN_FLAGS) $(IMAGE_INTERCEPTOR_SHA_TAG)
	cosign sign $(COSIGN_FLAGS) $(IMAGE_SCALER_VERSIONED_TAG)
	cosign sign $(COSIGN_FLAGS) $(IMAGE_SCALER_SHA_TAG)

##################################################
# Tool installation                              #
##################################################

$(LOCALBIN):
	mkdir -p $(LOCALBIN)

$(GOLANGCI_LINT): | $(LOCALBIN)
	$(call go-install-tool,golangci-lint,github.com/golangci/golangci-lint/v2/cmd/golangci-lint,$(GOLANGCI_LINT_VERSION))

$(KUSTOMIZE): | $(LOCALBIN)
	$(call go-install-tool,kustomize,sigs.k8s.io/kustomize/kustomize/v5,$(KUSTOMIZE_VERSION))

$(HELM): | $(LOCALBIN)
	$(call go-install-tool,helm,helm.sh/helm/v4/cmd/helm,$(HELM_VERSION))

define go-install-tool
@echo "Installing $(2)@$(3)"
@rm -f $(LOCALBIN)/$(1)
@GOBIN=$(LOCALBIN) go install $(2)@$(3)
@mv $(LOCALBIN)/$(1) $(LOCALBIN)/$(1)-$(3)
@ln -sf $(1)-$(3) $(LOCALBIN)/$(1)
endef
