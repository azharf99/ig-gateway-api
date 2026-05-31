# Stage 1: Build the Go application
FROM golang:1.26-alpine AS builder

WORKDIR /app

# Install build-base (needed for potential CGO dependencies, though we disable it)
RUN apk add --no-cache git

# Copy dependency manifests
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build statically-linked binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/api ./cmd/api

# Stage 2: Create a minimal production container
FROM alpine:latest

RUN apk add --no-cache ca-certificates tzdata ffmpeg font-dejavu

# Create non-root system user and group
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/api .

# Create uploads directory and configure ownership and permissions
RUN mkdir -p /app/uploads && \
    chown -R appuser:appgroup /app && \
    chmod 750 /app/uploads

EXPOSE 8080

# Switch to non-root user
USER appuser

ENTRYPOINT ["./api"]
