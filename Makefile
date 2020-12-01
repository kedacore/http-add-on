
.PHONY: gen-scaler
gen-scaler:
	protoc scaler/scaler.proto --go_out=plugins=grpc:externalscaler

.PHONY: build-scaler
build-scaler:
	go build -o bin/scaler ./scaler

.PHONY: build-interceptor
build-interceptor:
	go build -o bin/interceptor ./interceptor

.PHONY: build-operator
build-operator:
	cargo build --bin operator
