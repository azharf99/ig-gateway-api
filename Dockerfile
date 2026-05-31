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

# Install dependencies, su-exec to securely drop root privileges, and dos2unix for Windows compatibility
RUN apk add --no-cache ca-certificates tzdata ffmpeg font-dejavu su-exec dos2unix

# Create non-root system user and group with standard UID/GID 1000
RUN addgroup -g 1000 -S appgroup && adduser -u 1000 -S appuser -G appgroup

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/api .

# Copy entrypoint script and fix potential Windows line endings
COPY entrypoint.sh .
RUN dos2unix entrypoint.sh && chmod +x entrypoint.sh

# Create uploads directory and apply baseline permissions
RUN mkdir -p /app/uploads && \
    chown -R appuser:appgroup /app && \
    chmod 770 /app/uploads

EXPOSE 8080

ENTRYPOINT ["/app/entrypoint.sh"]
