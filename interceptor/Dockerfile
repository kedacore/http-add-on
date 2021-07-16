# adapted from Athens
# https://github.com/gomods/athens/blob/main/cmd/proxy/Dockerfile
ARG GOLANG_VERSION=1.16
ARG GOARCH=amd64
ARG GOOS=linux

FROM golang:${GOLANG_VERSION}-alpine AS builder

WORKDIR $GOPATH/src/github.com/kedacore/http-add-on

COPY go.mod go.mod
COPY go.sum go.sum
RUN go mod download

COPY . .

ENV GO111MODULE=on
ENV CGO_ENABLED=0
ENV GOOS=${GOOS}
ENV GOARCH=${GOARCH}
ENV GOPROXY="https://proxy.golang.org"
RUN go build -o /bin/interceptor ./interceptor

FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /bin/interceptor .
USER nonroot:nonroot
EXPOSE 8080

ENTRYPOINT ["/interceptor"]
