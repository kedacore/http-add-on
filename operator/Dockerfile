# Build the manager binary
FROM golang:1.13 as builder

WORKDIR /workspace
# Copy the Go Modules manifests
# NOTE: go.mod and go.sum are out of the build context when built from operator/makefile
# NOTE: run from root dir to make them work properly
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
# FIX: This only works when executing the makefile from the root, it'll not work if the operator/makefile is executed due to these files being out of the build context
COPY operator/main.go main.go
COPY operator/api/ api/
COPY operator/controllers/ controllers/

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -a -o manager main.go

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/manager .
USER nonroot:nonroot

ENTRYPOINT ["/manager"]
