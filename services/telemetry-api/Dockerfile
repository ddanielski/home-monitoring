# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install CA certificates for HTTPS
RUN apk --no-cache add ca-certificates

# Copy go mod files first for layer caching
COPY go.mod go.sum* ./
RUN go mod download

# Copy source code
COPY . .

# Build with security flags
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o /telemetry-api .

# Runtime stage - distroless for security
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /telemetry-api /telemetry-api

USER nonroot:nonroot

ENTRYPOINT ["/telemetry-api"]
