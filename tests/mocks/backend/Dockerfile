FROM golang:1.23.6 as build

WORKDIR /go/src/mocks/backend
COPY . .

RUN CGO_ENABLED=0 go build -o /go/bin/mocks/backend main.go

FROM gcr.io/distroless/static-debian12

COPY --from=build /go/bin/mocks/backend /
CMD ["/backend"]
