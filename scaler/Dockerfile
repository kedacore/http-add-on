# Build the adapter binary
FROM --platform=$BUILDPLATFORM ghcr.io/kedacore/build-tools:1.18.8 as builder

ARG VERSION=main
ARG GIT_COMMIT=HEAD

WORKDIR /workspace

COPY go.mod go.mod
COPY go.sum go.sum
RUN go mod download

COPY . .

# Build
# https://www.docker.com/blog/faster-multi-platform-builds-dockerfile-cross-compilation-guide/
ARG TARGETOS
ARG TARGETARCH
RUN VERSION=${VERSION} GIT_COMMIT=${GIT_COMMIT} TARGET_OS=$TARGETOS ARCH=$TARGETARCH make build-scaler

FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/bin/scaler .
# 65532 is numeric for nonroot
USER 65532:65532
EXPOSE 8080

ENTRYPOINT ["/scaler"]
