FROM golang:1.26-alpine AS builder
WORKDIR /build
COPY main.go go.mod go.sum ./
RUN go mod download
RUN CGO_ENABLED=0 go build -o /testapi main.go

FROM alpine:3.21
COPY --from=builder /testapi /testapi
EXPOSE 8089
CMD ["/testapi"]
