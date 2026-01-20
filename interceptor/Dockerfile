FROM --platform=${BUILDPLATFORM} ghcr.io/kedacore/keda-tools:1.25.5 as builder
WORKDIR /workspace
COPY go.* .
RUN go mod download
COPY . .
ARG VERSION=main
ARG GIT_COMMIT=HEAD
ARG TARGETOS
ARG TARGETARCH
RUN VERSION="${VERSION}" GIT_COMMIT="${GIT_COMMIT}" TARGET_OS="${TARGETOS}" ARCH="${TARGETARCH}" make build-interceptor

FROM gcr.io/distroless/static:nonroot
COPY --from=builder /workspace/bin/interceptor /sbin/init
ENTRYPOINT ["/sbin/init"]
