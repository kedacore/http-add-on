# taken from Athens 
# https://github.com/gomods/athens/blob/main/cmd/proxy/Dockerfile
ARG GOLANG_VERSION=1.14
ARG ALPINE_VERSION=3.11.5

FROM golang:${GOLANG_VERSION}-alpine AS builder

WORKDIR $GOPATH/src/github.com/kedahttp/http-add-on

COPY . .

ENV GO111MODULE=on
ENV CGO_ENABLED=0
ENV GOPROXY="https://proxy.golang.org"
RUN go build -o /bin/scaler ./scaler

FROM alpine:${ALPINE_VERSION}

COPY --from=builder /bin/scaler /bin/scaler

EXPOSE 8080

ENTRYPOINT ["/bin/scaler"]
