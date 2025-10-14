FROM --platform=${BUILDPLATFORM} ghcr.io/kedacore/keda-tools:1.24.7 as builder
WORKDIR /workspace
COPY go.* .
RUN go mod download
COPY . .
ARG VERSION=main
ARG GIT_COMMIT=HEAD
ARG TARGETOS
ARG TARGETARCH
RUN VERSION="${VERSION}" GIT_COMMIT="${GIT_COMMIT}" TARGET_OS="${TARGETOS}" ARCH="${TARGETARCH}" make build-scaler

FROM gcr.io/distroless/static:nonroot
COPY --from=builder /workspace/bin/scaler /sbin/init
ENTRYPOINT ["/sbin/init"]
