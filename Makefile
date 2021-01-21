GIT_TAG?=$(shell git rev-parse --short HEAD)
SCALER_DOCKER_IMG?=arschles/keda-http-scaler:${GIT_TAG}
INTERCEPTOR_DOCKER_IMG?=arschles/keda-http-interceptor:${GIT_TAG}
OPERATOR_DOCKER_IMG?=arschles/keda-http-operator:${GIT_TAG}

.PHONY: gen-scaler
gen-scaler:
	protoc \
		--go_out=scaler/gen/ \
		--go_opt=paths=source_relative \
		--go-grpc_out=scaler/gen/ \
		--go-grpc_opt=paths=source_relative \
		scaler/scaler.proto

.PHONY: build-scaler
build-scaler:
	go build -o bin/scaler ./scaler

.PHONY: docker-build-scaler
docker-build-scaler:
	docker build -t ${SCALER_DOCKER_IMG} -f scaler/Dockerfile .

.PHONY: docker-push-scaler
docker-push-scaler: docker-build-scaler
	docker push ${SCALER_DOCKER_IMG}

.PHONY: build-interceptor
build-interceptor:
	go build -o bin/interceptor ./interceptor

.PHONY: docker-build-interceptor
docker-build-interceptor:
	docker build -t ${INTERCEPTOR_DOCKER_IMG} -f interceptor/Dockerfile .

.PHONY: docker-push-interceptor
docker-push-interceptor: docker-build-interceptor
	docker push ${INTERCEPTOR_DOCKER_IMG}

# .PHONY: build-operator
# build-operator-cli:
# 	cargo build --bin operator

.PHONY: build-operator
build-operator:
	go build -o bin/operator ./operator

.PHONY: docker-build-operator
docker-build-operator:
	docker build -t ${OPERATOR_DOCKER_IMG} -f operator/Dockerfile .

.PHONY: docker-push-operator
docker-push-operator: docker-build-operator
	docker push ${OPERATOR_DOCKER_IMG}

.PHONY: build-all
build-all: build-scaler build-interceptor build-operator

.PHONY: docker-build-all
docker-build-all: docker-build-scaler docker-build-interceptor docker-build-operator

.PHONY: docker-push-all
docker-push-all: docker-push-scaler docker-push-interceptor docker-push-operator
