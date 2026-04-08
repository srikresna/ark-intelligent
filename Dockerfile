# ===========================================================================
# ARK Intelligence v4 — Multi-stage Docker build
# Stage 1: Build Go binary with CGO disabled (static linking)
# Stage 2: Minimal Alpine runtime with ca-certs and timezone data
# ===========================================================================

# ---------------------------------------------------------------------------
# Stage 1: Builder
# ---------------------------------------------------------------------------
FROM golang:1.22-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /build

# Copy all source code
COPY . .

# Generate go.sum and download dependencies
RUN go mod tidy && go mod download

# Build static binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w -X main.version=4.0.0" \
    -trimpath \
    -o /build/ark-intelligent \
    ./cmd/bot

# ---------------------------------------------------------------------------
# Stage 2: Runtime
# ---------------------------------------------------------------------------
FROM alpine:3.19

# Install runtime dependencies + Python for chart rendering
RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    curl \
    python3 \
    py3-pip \
    py3-numpy \
    py3-pandas \
    py3-matplotlib \
    && rm -rf /var/cache/apk/*

# Install Python chart dependencies not available as Alpine packages
RUN pip3 install --no-cache-dir --break-system-packages mplfinance

# Create non-root user
RUN addgroup -S botgroup && adduser -S botuser -G botgroup

# Create data directory
RUN mkdir -p /app/data && chown -R botuser:botgroup /app

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/ark-intelligent /app/ark-intelligent

# Copy Python chart/engine scripts
COPY scripts/ /app/scripts/

# Set timezone and script discovery path
ENV TZ=Asia/Jakarta
ENV SCRIPTS_DIR=/app/scripts

# Health check — HTTP liveness probe via /health endpoint
HEALTHCHECK --interval=60s --timeout=5s --start-period=30s --retries=3 \
    CMD curl -sf http://localhost:8080/health || exit 1

# Switch to non-root user
USER botuser

# Note: For persistent storage, configure a Railway volume mounted at /app/data
# See: https://docs.railway.com/reference/volumes

ENTRYPOINT ["/app/ark-intelligent"]
