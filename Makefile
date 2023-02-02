##################################################
# Variables                                      #
##################################################
SHELL           = /bin/bash

IMAGE_REGISTRY 	?= ghcr.io
IMAGE_REPO     	?= kedacore/http-add-on
VERSION 		?= main

IMAGE_OPERATOR_BASE 	?= ${IMAGE_REGISTRY}/${IMAGE_REPO}/operator
IMAGE_INTERCEPTOR_BASE	?= ${IMAGE_REGISTRY}/${IMAGE_REPO}/interceptor
IMAGE_SCALER_BASE		?= ${IMAGE_REGISTRY}/${IMAGE_REPO}/scaler

IMAGE_OPERATOR_VERSION		?= ${IMAGE_OPERATOR_BASE}:$(VERSION)
IMAGE_INTERCEPTOR_VERSION	?= ${IMAGE_INTERCEPTOR_BASE}:$(VERSION)
IMAGE_SCALER_VERSION		?= ${IMAGE_SCALER_BASE}:$(VERSION)

IMAGE_OPERATOR_SHA		?= ${IMAGE_OPERATOR_BASE}:$(GIT_COMMIT_SHORT)
IMAGE_INTERCEPTOR_SHA	?= ${IMAGE_INTERCEPTOR_BASE}:$(GIT_COMMIT_SHORT)
IMAGE_SCALER_SHA		?= ${IMAGE_SCALER_BASE}:$(GIT_COMMIT_SHORT)

ARCH       ?=amd64
CGO        ?=0
TARGET_OS  ?=linux

BUILD_PLATFORMS ?= linux/amd64,linux/arm64
OUTPUT_TYPE     ?= registry

GO_BUILD_VARS= GO111MODULE=on CGO_ENABLED=$(CGO) GOOS=$(TARGET_OS) GOARCH=$(ARCH)
GO_LDFLAGS="-X github.com/kedacore/http-add-on/pkg/build.version=${VERSION} -X github.com/kedacore/http-add-on/pkg/build.gitCommit=${GIT_COMMIT}"

GIT_COMMIT  ?= $(shell git rev-list -1 HEAD)
GIT_COMMIT_SHORT  ?= $(shell git rev-parse --short HEAD)

# Build targets

build-operator: proto-gen
	${GO_BUILD_VARS} go build -ldflags $(GO_LDFLAGS) -a -o bin/operator ./operator

build-interceptor: proto-gen
	${GO_BUILD_VARS} go build -ldflags $(GO_LDFLAGS) -a -o bin/interceptor ./interceptor

build-scaler: proto-gen
	${GO_BUILD_VARS} go build -ldflags $(GO_LDFLAGS) -a -o bin/scaler ./scaler

build: build-operator build-interceptor build-scaler

# Test targets
test: fmt vet
	go test ./...

e2e-test:
	./tests/e2e-test.sh

# Docker targets
docker-build-operator:
	DOCKER_BUILDKIT=1 docker build . -t ${IMAGE_OPERATOR_VERSION} -t ${IMAGE_OPERATOR_SHA} -f operator/Dockerfile --build-arg VERSION=${VERSION} --build-arg GIT_COMMIT=${GIT_COMMIT}

docker-build-interceptor:
	DOCKER_BUILDKIT=1 docker build . -t ${IMAGE_INTERCEPTOR_VERSION} -t ${IMAGE_INTERCEPTOR_SHA} -f interceptor/Dockerfile --build-arg VERSION=${VERSION} --build-arg GIT_COMMIT=${GIT_COMMIT}

docker-build-scaler:
	DOCKER_BUILDKIT=1 docker build . -t ${IMAGE_SCALER_VERSION} -t ${IMAGE_SCALER_SHA} -f scaler/Dockerfile --build-arg VERSION=${VERSION} --build-arg GIT_COMMIT=${GIT_COMMIT}

docker-build: docker-build-operator docker-build-interceptor docker-build-scaler

docker-publish: docker-build ## Push images on to Container Registry (default: ghcr.io).
	docker push $(IMAGE_OPERATOR_VERSION)
	docker push $(IMAGE_OPERATOR_SHA)
	docker push $(IMAGE_INTERCEPTOR_VERSION)
	docker push $(IMAGE_INTERCEPTOR_SHA)
	docker push $(IMAGE_SCALER_VERSION)
	docker push $(IMAGE_SCALER_SHA)

publish-operator-multiarch:
	docker buildx build --output=type=${OUTPUT_TYPE} --platform=${BUILD_PLATFORMS} . -t ${IMAGE_OPERATOR_VERSION} -t ${IMAGE_OPERATOR_SHA} -f operator/Dockerfile --build-arg VERSION=${VERSION} --build-arg GIT_COMMIT=${GIT_COMMIT}

publish-interceptor-multiarch:
	docker buildx build --output=type=${OUTPUT_TYPE} --platform=${BUILD_PLATFORMS} . -t ${IMAGE_INTERCEPTOR_VERSION} -t ${IMAGE_INTERCEPTOR_SHA} -f interceptor/Dockerfile --build-arg VERSION=${VERSION} --build-arg GIT_COMMIT=${GIT_COMMIT}

publish-scaler-multiarch:
	docker buildx build --output=type=${OUTPUT_TYPE} --platform=${BUILD_PLATFORMS} . -t ${IMAGE_SCALER_VERSION} -t ${IMAGE_SCALER_SHA} -f scaler/Dockerfile --build-arg VERSION=${VERSION} --build-arg GIT_COMMIT=${GIT_COMMIT}

publish-multiarch: publish-operator-multiarch publish-interceptor-multiarch publish-scaler-multiarch

# Development

manifests: controller-gen ## Generate ClusterRole and CustomResourceDefinition objects for core componenets.
	$(CONTROLLER_GEN) crd:crdVersions=v1 rbac:roleName=keda-http-add-on paths="./..." output:crd:artifacts:config=config/crd/bases

verify-manifests:
	./hack/verify-manifests.sh

fmt: ## Run go fmt against code.
	go fmt ./...

vet: ## Run go vet against code.
	go vet ./...

pre-commit: ## Run static-checks.
	pre-commit run --all-files

proto-gen: protoc-gen-go ## Scaler protobuffers
	protoc --proto_path=proto scaler.proto --go_out=proto --go-grpc_out=proto

CONTROLLER_GEN = $(shell pwd)/bin/controller-gen
controller-gen: ## Download controller-gen locally if necessary.
	GOBIN=$(shell pwd)/bin go install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.10.0

KUSTOMIZE = $(shell pwd)/bin/kustomize
kustomize: ## Download kustomize locally if necessary.
	GOBIN=$(shell pwd)/bin go install sigs.k8s.io/kustomize/kustomize/v4@v4.5.7

protoc-gen-go: ## Download protoc-gen-go
	go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.28.1
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.2.0

deploy: manifests kustomize ## Deploy to the K8s cluster specified in ~/.kube/config.
	cd config/interceptor && \
	$(KUSTOMIZE) edit set image ghcr.io/kedacore/http-add-on-interceptor=${IMAGE_INTERCEPTOR_VERSION}

	cd config/scaler && \
	$(KUSTOMIZE) edit set image ghcr.io/kedacore/http-add-on-scaler=${IMAGE_SCALER_VERSION}

	cd config/operator && \
	$(KUSTOMIZE) edit set image ghcr.io/kedacore/http-add-on-operator=${IMAGE_OPERATOR_VERSION}

	@sed -i".out" -e 's@version:[ ].*@version: $(VERSION)@g' config/default/kustomize-config/metadataLabelTransformer.yaml
	rm -rf config/default/kustomize-config/metadataLabelTransformer.yaml.out
	$(KUSTOMIZE) build config/default | kubectl apply -f -

undeploy:
	$(KUSTOMIZE) build config/default | kubectl delete -f -
