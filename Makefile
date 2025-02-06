##################################################
# Variables                                      #
##################################################
SHELL           = /bin/bash

IMAGE_REGISTRY 	?= navigation-docker.artifactory.teslamotors.com
IMAGE_REPO     	?= kedacore
VERSION 		?= header-based-routing

IMAGE_OPERATOR 		?= ${IMAGE_REGISTRY}/${IMAGE_REPO}/http-add-on-operator
IMAGE_INTERCEPTOR	?= ${IMAGE_REGISTRY}/${IMAGE_REPO}/http-add-on-interceptor
IMAGE_SCALER		?= ${IMAGE_REGISTRY}/${IMAGE_REPO}/http-add-on-scaler

IMAGE_OPERATOR_VERSIONED_TAG	?= ${IMAGE_OPERATOR}:$(VERSION)
IMAGE_INTERCEPTOR_VERSIONED_TAG	?= ${IMAGE_INTERCEPTOR}:$(VERSION)
IMAGE_SCALER_VERSIONED_TAG		?= ${IMAGE_SCALER}:$(VERSION)

IMAGE_OPERATOR_SHA_TAG		?= ${IMAGE_OPERATOR}:$(GIT_COMMIT_SHORT)
IMAGE_INTERCEPTOR_SHA_TAG	?= ${IMAGE_INTERCEPTOR}:$(GIT_COMMIT_SHORT)
IMAGE_SCALER_SHA_TAG		?= ${IMAGE_SCALER}:$(GIT_COMMIT_SHORT)

ARCH       ?=amd64
CGO        ?=0
TARGET_OS  ?=linux

BUILD_PLATFORMS ?= linux/amd64,linux/arm64
OUTPUT_TYPE     ?= registry

GO_BUILD_VARS= GO111MODULE=on CGO_ENABLED=$(CGO) GOOS=$(TARGET_OS) GOARCH=$(ARCH)
GO_LDFLAGS="-X github.com/kedacore/http-add-on/pkg/build.version=${VERSION} -X github.com/kedacore/http-add-on/pkg/build.gitCommit=${GIT_COMMIT}"

GIT_COMMIT  ?= $(shell git rev-list -1 HEAD)
GIT_COMMIT_SHORT  ?= $(shell git rev-parse --short HEAD)

COSIGN_FLAGS ?= -y -a GIT_HASH=${GIT_COMMIT} -a GIT_VERSION=${VERSION} -a BUILD_DATE=${DATE}

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

# Build targets

build-operator:
	${GO_BUILD_VARS} go build -ldflags $(GO_LDFLAGS) -trimpath -a -o bin/operator ./operator

build-interceptor:
	${GO_BUILD_VARS} go build -ldflags $(GO_LDFLAGS) -trimpath -a -o bin/interceptor ./interceptor

build-scaler:
	${GO_BUILD_VARS} go build -ldflags $(GO_LDFLAGS) -trimpath -a -o bin/scaler ./scaler

build: build-operator build-interceptor build-scaler

# generate certs for local unit and e2e tests
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
	rm -r certs || true

# Test targets
test: fmt vet test-certs
	go test ./...

e2e-test:
	go run -tags e2e ./tests/run-all.go

e2e-test-setup:
	ONLY_SETUP=true go run -tags e2e ./tests/run-all.go

e2e-test-local:
	SKIP_SETUP=true go run -tags e2e ./tests/run-all.go

# Docker targets
docker-build-operator:
	DOCKER_BUILDKIT=1 docker build . -t ${IMAGE_OPERATOR_VERSIONED_TAG} -t ${IMAGE_OPERATOR_SHA_TAG} -f operator/Dockerfile --build-arg VERSION=${VERSION} --build-arg GIT_COMMIT=${GIT_COMMIT}

docker-build-interceptor:
	DOCKER_BUILDKIT=1 docker build . -t ${IMAGE_INTERCEPTOR_VERSIONED_TAG} -t ${IMAGE_INTERCEPTOR_SHA_TAG} -f interceptor/Dockerfile --build-arg VERSION=${VERSION} --build-arg GIT_COMMIT=${GIT_COMMIT}

docker-build-scaler:
	DOCKER_BUILDKIT=1 docker build . -t ${IMAGE_SCALER_VERSIONED_TAG} -t ${IMAGE_SCALER_SHA_TAG} -f scaler/Dockerfile --build-arg VERSION=${VERSION} --build-arg GIT_COMMIT=${GIT_COMMIT}

docker-build: docker-build-operator docker-build-interceptor docker-build-scaler

docker-publish: docker-build ## Push images on to Container Registry (default: ghcr.io).
	docker push $(IMAGE_OPERATOR_VERSIONED_TAG)
	docker push $(IMAGE_OPERATOR_SHA_TAG)
	docker push $(IMAGE_INTERCEPTOR_VERSIONED_TAG)
	docker push $(IMAGE_INTERCEPTOR_SHA_TAG)
	docker push $(IMAGE_SCALER_VERSIONED_TAG)
	docker push $(IMAGE_SCALER_SHA_TAG)

publish-operator-multiarch:
	docker buildx build --output=type=${OUTPUT_TYPE} --platform=${BUILD_PLATFORMS} . -t ${IMAGE_OPERATOR_VERSIONED_TAG} -t ${IMAGE_OPERATOR_SHA_TAG} -f operator/Dockerfile --build-arg VERSION=${VERSION} --build-arg GIT_COMMIT=${GIT_COMMIT}

publish-interceptor-multiarch:
	docker buildx build --output=type=${OUTPUT_TYPE} --platform=${BUILD_PLATFORMS} . -t ${IMAGE_INTERCEPTOR_VERSIONED_TAG} -t ${IMAGE_INTERCEPTOR_SHA_TAG} -f interceptor/Dockerfile --build-arg VERSION=${VERSION} --build-arg GIT_COMMIT=${GIT_COMMIT}

publish-scaler-multiarch:
	docker buildx build --output=type=${OUTPUT_TYPE} --platform=${BUILD_PLATFORMS} . -t ${IMAGE_SCALER_VERSIONED_TAG} -t ${IMAGE_SCALER_SHA_TAG} -f scaler/Dockerfile --build-arg VERSION=${VERSION} --build-arg GIT_COMMIT=${GIT_COMMIT}

publish-multiarch: publish-operator-multiarch publish-interceptor-multiarch publish-scaler-multiarch

release: manifests kustomize ## Produce new KEDA Http Add-on release in keda-http-add-on-$(VERSION).yaml file.
	cd config/interceptor && \
	$(KUSTOMIZE) edit set image ghcr.io/kedacore/http-add-on-interceptor=${IMAGE_INTERCEPTOR_VERSIONED_TAG}
	cd config/scaler && \
	$(KUSTOMIZE) edit set image ghcr.io/kedacore/http-add-on-scaler=${IMAGE_SCALER_VERSIONED_TAG}
	cd config/operator && \
	$(KUSTOMIZE) edit set image ghcr.io/kedacore/http-add-on-operator=${IMAGE_OPERATOR_VERSIONED_TAG}
	$(KUSTOMIZE) build config/default > keda-http-add-on-$(VERSION).yaml
	$(KUSTOMIZE) build config/crd     > keda-http-add-on-$(VERSION)-crds.yaml

# Development

generate: codegen mockgen manifests  ## Generate code, manifests, and mocks.

verify: verify-codegen verify-mockgen verify-manifests ## Verify code, manifests, and mocks.

codegen: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile='hack/boilerplate.go.txt' paths='./...'
	./hack/update-codegen.sh

verify-codegen: ## Verify code is up to date.
	./hack/verify-codegen.sh

manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) crd rbac:roleName='operator' webhook paths='./operator/...' output:crd:artifacts:config='config/crd/bases' output:rbac:artifacts:config='config/operator'
	$(CONTROLLER_GEN) crd rbac:roleName='scaler' webhook paths='./scaler/...' output:rbac:artifacts:config='config/scaler'
	$(CONTROLLER_GEN) crd rbac:roleName='interceptor' webhook paths='./interceptor/...' output:rbac:artifacts:config='config/interceptor'

verify-manifests: ## Verify manifests are up to date.
	./hack/verify-manifests.sh

sign-images: ## Sign KEDA images published on GitHub Container Registry
	COSIGN_EXPERIMENTAL=1 cosign sign ${COSIGN_FLAGS} $(IMAGE_OPERATOR_VERSIONED_TAG)
	COSIGN_EXPERIMENTAL=1 cosign sign ${COSIGN_FLAGS} $(IMAGE_OPERATOR_SHA_TAG)
	COSIGN_EXPERIMENTAL=1 cosign sign ${COSIGN_FLAGS} $(IMAGE_INTERCEPTOR_VERSIONED_TAG)
	COSIGN_EXPERIMENTAL=1 cosign sign ${COSIGN_FLAGS} $(IMAGE_INTERCEPTOR_SHA_TAG)
	COSIGN_EXPERIMENTAL=1 cosign sign ${COSIGN_FLAGS} $(IMAGE_SCALER_VERSIONED_TAG)
	COSIGN_EXPERIMENTAL=1 cosign sign ${COSIGN_FLAGS} $(IMAGE_SCALER_SHA_TAG)

mockgen: ## Generate mock implementations of Go interfaces.
	./hack/update-mockgen.sh

verify-mockgen: ## Verify mocks are up to date.
	./hack/verify-mockgen.sh

fmt: ## Run go fmt against code.
	go fmt ./...

vet: ## Run go vet against code.
	go vet ./...

lint: ## Run golangci-lint against code.
	golangci-lint run

pre-commit: ## Run static-checks.
	pre-commit run --all-files

CONTROLLER_GEN = $(shell pwd)/bin/controller-gen
controller-gen: ## Download controller-gen locally if necessary.
	GOBIN=$(shell pwd)/bin go install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.15.0

KUSTOMIZE = $(shell pwd)/bin/kustomize
kustomize: ## Download kustomize locally if necessary.
	GOBIN=$(shell pwd)/bin go install sigs.k8s.io/kustomize/kustomize/v5

install: manifests kustomize ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | kubectl apply -f -

deploy: manifests kustomize ## Deploy to the K8s cluster specified in ~/.kube/config.
	cd config/interceptor && \
	$(KUSTOMIZE) edit set image ghcr.io/kedacore/http-add-on-interceptor=${IMAGE_INTERCEPTOR_VERSIONED_TAG}

	cd config/interceptor && \
	$(KUSTOMIZE) edit add patch --path e2e-test/otel/deployment.yaml --group apps --kind Deployment --name interceptor --version v1

	cd config/interceptor && \
	$(KUSTOMIZE) edit add patch --path e2e-test/otel/scaledobject.yaml --group keda.sh --kind ScaledObject --name interceptor --version v1alpha1

	cd config/interceptor && \
	$(KUSTOMIZE) edit add patch --path e2e-test/tls/deployment.yaml --group apps --kind Deployment --name interceptor --version v1

	cd config/interceptor && \
	$(KUSTOMIZE) edit add patch --path e2e-test/tls/proxy.service.yaml --kind Service --name interceptor-proxy --version v1

	cd config/scaler && \
	$(KUSTOMIZE) edit set image ghcr.io/kedacore/http-add-on-scaler=${IMAGE_SCALER_VERSIONED_TAG}

	cd config/scaler && \
	$(KUSTOMIZE) edit add patch --path e2e-test/otel/deployment.yaml --group apps --kind Deployment --name scaler --version v1

	cd config/operator && \
	$(KUSTOMIZE) edit set image ghcr.io/kedacore/http-add-on-operator=${IMAGE_OPERATOR_VERSIONED_TAG}

	$(KUSTOMIZE) build config/default | kubectl apply -f -

undeploy:
	$(KUSTOMIZE) build config/default | kubectl delete -f -
