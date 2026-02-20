# syntax=docker/dockerfile:1

# K6 binary
FROM grafana/k6:latest AS k6

# Build stage
FROM golang:1.26-alpine AS builder

RUN apk add --no-cache git gcc musl-dev

WORKDIR /app

# Copy source code
COPY . .

# Download dependencies
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod tidy && go mod download

# Build binaries
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /api ./cmd/api && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /migrate ./cmd/migrate && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /seed ./cmd/seed

# Runtime stage
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# Copy K6 binary
COPY --from=k6 /usr/bin/k6 /usr/bin/k6

# Copy Go binaries
COPY --from=builder /api /app/api
COPY --from=builder /migrate /app/migrate
COPY --from=builder /seed /app/seed

# Copy migrations
COPY --from=builder /app/migrations /app/migrations

# Create directories
RUN mkdir -p /app/k6-scripts

EXPOSE 8080

CMD ["/app/api"]
