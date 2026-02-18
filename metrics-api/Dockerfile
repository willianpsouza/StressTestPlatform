FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /metrics-api .

FROM alpine:3.21
RUN apk add --no-cache ca-certificates
COPY --from=builder /metrics-api /app/metrics-api
WORKDIR /app
EXPOSE 8081
CMD ["./metrics-api"]
