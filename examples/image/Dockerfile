FROM --platform=$BUILDPLATFORM golang:1.20 as build-env

WORKDIR /app
COPY main.go main.go

ARG TARGETOS
ARG TARGETARCH
RUN GO111MODULE=on CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o /go/bin/server main.go

FROM gcr.io/distroless/base
COPY --from=build-env /go/bin/server /
EXPOSE 8080

ENTRYPOINT ["/server"]
