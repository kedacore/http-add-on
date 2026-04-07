##################################################
# Variables                                      #
##################################################
SHELL          = /bin/bash
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

KO_RELEASE_PLATFORMS ?= linux/amd64,linux/arm64

# renovate: datasource=helm depName=cert-manager registryUrl=https://charts.jetstack.io
CERT_MANAGER_VERSION ?= v1.20.1
# renovate: datasource=helm depName=jaeger registryUrl=https://jaegertracing.github.io/helm-charts
JAEGER_VERSION ?= 4.7.0
# renovate: datasource=helm depName=keda registryUrl=https://kedacore.github.io/charts
KEDA_VERSION ?= 2.19.0
# renovate: datasource=helm depName=opentelemetry-collector registryUrl=https://open-telemetry.github.io/opentelemetry-helm-charts
OTEL_COLLECTOR_VERSION ?= 0.147.1

COSIGN_FLAGS ?= -y -a GIT_HASH=$(GIT_COMMIT) -a GIT_VERSION=$(VERSION) -a BUILD_DATE=$(DATE)

## Tool Binaries
CONTROLLER_GEN ?= go tool controller-gen

# TODO(v1): remove DOMAINS, ABC_DOMAINS, and the cert targets below when removing the legacy e2e tests.
define DOMAINS
basicConstraints=CA:FALSE
keyUsage = digitalSignature, nonRepudiation, keyEncipherment, dataEncipherment
subjectAltName = @alt_names
[alt_names]
DNS.1 = localhost
DNS.2 = *.keda
DNS.3 = *.interceptor-tls-test-ns
endef
export DOMAINS

define ABC_DOMAINS
basicConstraints=CA:FALSE
keyUsage = digitalSignature, nonRepudiation, keyEncipherment, dataEncipherment
subjectAltName = @alt_names
[alt_names]
DNS.1 = abc
endef
export ABC_DOMAINS

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

# TODO(v1): remove cert targets below when removing the legacy e2e tests.
rootca-test-certs:
	mkdir -p certs
	openssl req -x509 -nodes -new -sha256 -days 1024 -newkey rsa:2048 -keyout certs/RootCA.key -out certs/RootCA.pem -subj "/C=US/CN=Keda-Root-CA"
	openssl x509 -outform pem -in certs/RootCA.pem -out certs/RootCA.crt

test-certs: rootca-test-certs
	echo "$$DOMAINS" > certs/domains.ext
	openssl req -new -nodes -newkey rsa:2048 -keyout certs/tls.key -out certs/tls.csr -subj "/C=US/ST=KedaState/L=KedaCity/O=Keda-Certificates/CN=keda.local"
	openssl x509 -req -sha256 -days 1024 -in certs/tls.csr -CA certs/RootCA.pem -CAkey certs/RootCA.key -CAcreateserial -extfile certs/domains.ext -out certs/tls.crt
	echo "$$ABC_DOMAINS" > certs/abc_domains.ext
	openssl req -new -nodes -newkey rsa:2048 -keyout certs/abc.tls.key -out certs/abc.tls.csr -subj "/C=US/ST=KedaState/L=KedaCity/O=Keda-Certificates/CN=abc"
	openssl x509 -req -sha256 -days 1024 -in certs/abc.tls.csr -CA certs/RootCA.pem -CAkey certs/RootCA.key -CAcreateserial -extfile certs/abc_domains.ext -out certs/abc.tls.crt

clean-test-certs:
	rm -rf certs

.PHONY: test
test:
	go test ./...

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
	gotestsum --rerun-fails=2 --format=github-actions --packages="./test/e2e/..." -- -tags e2e -p 1 -count=1 -timeout 30m -v -parallel 4

e2e-deps: e2e-deps-cert-manager e2e-deps-jaeger e2e-deps-keda e2e-deps-otel-collector ## Install all e2e dependencies

e2e-deps-cert-manager:
	helm repo add jetstack https://charts.jetstack.io --force-update
	helm upgrade --install cert-manager jetstack/cert-manager \
		--namespace cert-manager --create-namespace \
		-f test/fixtures/cert-manager-values.yaml \
		--version $(CERT_MANAGER_VERSION) --wait

e2e-deps-jaeger:
	helm repo add jaegertracing https://jaegertracing.github.io/helm-charts --force-update
	helm upgrade --install jaeger jaegertracing/jaeger \
		--namespace jaeger --create-namespace \
		--version $(JAEGER_VERSION) --wait

e2e-deps-keda:
	helm repo add kedacore https://kedacore.github.io/charts --force-update
	helm upgrade --install keda kedacore/keda \
		--namespace keda --create-namespace \
		--version $(KEDA_VERSION) --wait

e2e-deps-otel-collector:
	helm repo add open-telemetry https://open-telemetry.github.io/opentelemetry-helm-charts --force-update
	helm upgrade --install opentelemetry-collector open-telemetry/opentelemetry-collector \
		--namespace open-telemetry-system --create-namespace \
		-f test/fixtures/otel-values.yaml \
		--version $(OTEL_COLLECTOR_VERSION) --wait

e2e-setup: e2e-deps deploy ## Full e2e setup: install deps + deploy http-add-on

##################################################
# Code generation & manifests                    #
##################################################

generate: codegen manifests  ## Generate code and manifests.

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

fmt:
	golangci-lint fmt

lint:
	golangci-lint run

lint-fix:
	golangci-lint run --fix

pre-commit: ## Run static-checks.
	pre-commit run --all-files

##################################################
# Deployment (local cluster)                     #
##################################################

install:
	kustomize build config/crd | kubectl apply -f -

deploy:
	kustomize build config/default | ko apply -f -

deploy-e2e:
	kustomize build config/e2e | ko apply -f -

deploy-operator:
	kustomize build config/operator | ko apply -f -

deploy-interceptor:
	kustomize build config/interceptor | ko apply -f -

deploy-scaler:
	kustomize build config/scaler | ko apply -f -

undeploy:
	kustomize build config/default | ko delete -f - || true

##################################################
# Publish, release & signing                     #
##################################################

publish-operator:
	# --bare preserves image names like ghcr.io/kedacore/http-add-on-operator
	KO_DOCKER_REPO=$(IMAGE_OPERATOR) ko build --bare --platform=$(KO_RELEASE_PLATFORMS) --tags=$(VERSION),$(GIT_COMMIT_SHORT) ./operator

publish-interceptor:
	KO_DOCKER_REPO=$(IMAGE_INTERCEPTOR) ko build --bare --platform=$(KO_RELEASE_PLATFORMS) --tags=$(VERSION),$(GIT_COMMIT_SHORT) ./interceptor

publish-scaler:
	KO_DOCKER_REPO=$(IMAGE_SCALER) ko build --bare --platform=$(KO_RELEASE_PLATFORMS) --tags=$(VERSION),$(GIT_COMMIT_SHORT) ./scaler

publish: publish-operator publish-interceptor publish-scaler

release: manifests ## Produce new KEDA Http Add-on release in keda-add-ons-http-$(VERSION).yaml file.
	kustomize build config/crd > keda-add-ons-http-$(VERSION).yaml
	echo '---' >> keda-add-ons-http-$(VERSION).yaml
	kustomize build config/operator | KO_DOCKER_REPO=$(IMAGE_OPERATOR) ko resolve --bare --platform=$(KO_RELEASE_PLATFORMS) --tags=$(VERSION) -f - >> keda-add-ons-http-$(VERSION).yaml
	echo '---' >> keda-add-ons-http-$(VERSION).yaml
	kustomize build config/interceptor | KO_DOCKER_REPO=$(IMAGE_INTERCEPTOR) ko resolve --bare --platform=$(KO_RELEASE_PLATFORMS) --tags=$(VERSION) -f - >> keda-add-ons-http-$(VERSION).yaml
	echo '---' >> keda-add-ons-http-$(VERSION).yaml
	kustomize build config/scaler | KO_DOCKER_REPO=$(IMAGE_SCALER) ko resolve --bare --platform=$(KO_RELEASE_PLATFORMS) --tags=$(VERSION) -f - >> keda-add-ons-http-$(VERSION).yaml
	kustomize build config/crd > keda-add-ons-http-$(VERSION)-crds.yaml

sign-images: ## Sign KEDA images published on GitHub Container Registry
	cosign sign $(COSIGN_FLAGS) $(IMAGE_OPERATOR_VERSIONED_TAG)
	cosign sign $(COSIGN_FLAGS) $(IMAGE_OPERATOR_SHA_TAG)
	cosign sign $(COSIGN_FLAGS) $(IMAGE_INTERCEPTOR_VERSIONED_TAG)
	cosign sign $(COSIGN_FLAGS) $(IMAGE_INTERCEPTOR_SHA_TAG)
	cosign sign $(COSIGN_FLAGS) $(IMAGE_SCALER_VERSIONED_TAG)
	cosign sign $(COSIGN_FLAGS) $(IMAGE_SCALER_SHA_TAG)
