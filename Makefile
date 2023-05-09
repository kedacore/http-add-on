##################################################
# Variables                                      #
##################################################
SHELL           = /bin/bash

# IMAGE_REGISTRY 	?= ghcr.io
# IMAGE_REPO     	?= kedacore
#DEBUG
IMAGE_REPO     	?= swtrasformer
VERSION 		?= main

# IMAGE_OPERATOR 	?= ${IMAGE_REGISTRY}/${IMAGE_REPO}/http-add-on-operator
# IMAGE_INTERCEPTOR	?= ${IMAGE_REGISTRY}/${IMAGE_REPO}/http-add-on-interceptor
# IMAGE_SCALER		?= ${IMAGE_REGISTRY}/${IMAGE_REPO}/http-add-on-scaler
# DEBUG
IMAGE_OPERATOR 		?= ${IMAGE_REPO}/http-add-on-operator
IMAGE_INTERCEPTOR	?= ${IMAGE_REPO}/http-add-on-interceptor
IMAGE_SCALER		?= ${IMAGE_REPO}/http-add-on-scaler

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
	DOCKER_BUILDKIT=1 docker build . -t ${IMAGE_OPERATOR_VERSIONED_TAG} -t ${IMAGE_OPERATOR_SHA_TAG} -f operator/Dockerfile --build-arg VERSION=${VERSION} --build-arg GIT_COMMIT=${GIT_COMMIT}

docker-build-interceptor:
	DOCKER_BUILDKIT=1 docker build . -t ${IMAGE_INTERCEPTOR_VERSIONED_TAG} -t ${IMAGE_INTERCEPTOR_SHA_TAG} -f interceptor/Dockerfile --build-arg VERSION=${VERSION} --build-arg GIT_COMMIT=${GIT_COMMIT}

docker-build-scaler:
	DOCKER_BUILDKIT=1 docker build . -t ${IMAGE_SCALER_VERSIONED_TAG} -t ${IMAGE_SCALER_SHA_TAG} -f scaler/Dockerfile --build-arg VERSION=${VERSION} --build-arg GIT_COMMIT=${GIT_COMMIT}

#DEBUG
docker-build-operator-debug:
	DOCKER_BUILDKIT=1 docker build . -t ${IMAGE_OPERATOR_VERSIONED_TAG}-debug -f operator/Dockerfile.debug --build-arg VERSION=${VERSION} --build-arg GIT_COMMIT=${GIT_COMMIT}

docker-build-interceptor-debug:
	DOCKER_BUILDKIT=1 docker build . -t ${IMAGE_INTERCEPTOR_VERSIONED_TAG}-debug -f interceptor/Dockerfile.debug --build-arg VERSION=${VERSION} --build-arg GIT_COMMIT=${GIT_COMMIT}

docker-build-scaler-debug:
	DOCKER_BUILDKIT=1 docker build . -t ${IMAGE_SCALER_VERSIONED_TAG}-debug -f scaler/Dockerfile.debug --build-arg VERSION=${VERSION} --build-arg GIT_COMMIT=${GIT_COMMIT}

docker-build: docker-build-operator docker-build-interceptor docker-build-scaler
docker-build-debug: docker-build-operator-debug docker-build-interceptor-debug docker-build-scaler-debug

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

# Development

generate: codegen manifests ## Generate code and manifests.

verify: verify-codegen verify-manifests ## Verify code and manifests.

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
	./hack/verify-codegen.sh

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
	$(KUSTOMIZE) edit set image ghcr.io/kedacore/http-add-on-interceptor=${IMAGE_INTERCEPTOR_VERSIONED_TAG}

	cd config/scaler && \
	$(KUSTOMIZE) edit set image ghcr.io/kedacore/http-add-on-scaler=${IMAGE_SCALER_VERSIONED_TAG}

	cd config/operator && \
	$(KUSTOMIZE) edit set image ghcr.io/kedacore/http-add-on-operator=${IMAGE_OPERATOR_VERSIONED_TAG}

	cat <<< $$(sed -E 's|(^[[:space:]]+app\.kubernetes\.io/version:).*|\1 $(VERSION)|g' config/default/kustomization.yaml) > config/default/kustomization.yaml
	$(KUSTOMIZE) build config/default | kubectl apply -f -

undeploy:
	$(KUSTOMIZE) build config/default | kubectl delete -f -
