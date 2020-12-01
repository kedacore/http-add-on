
.PHONY: genscaler
gen-scaler:
	protoc scaler/scaler.proto --go_out=plugins=grpc:externalscaler

.PHONY: scaler
build-scaler:
	go build -o bin/scaler ./scaler
