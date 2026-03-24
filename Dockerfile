# Stage 1: Build
FROM golang:1.24-bookworm@sha256:1a6d4452c65dea36aac2e2d606b01b4a029ec90cc1ae53890540ce6173ea77ac AS builder  # 1.24-bookworm

WORKDIR /src

# Copy dependency manifests first for layer caching.
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build a static binary.
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /gateway ./cmd/gateway

# Stage 2: Runtime — distroless contains only the binary, no shell or toolchain.
FROM gcr.io/distroless/static@sha256:e3f945647ffb95b5839c07038d64f9811adf17308b9121d8a2b87b6a22a80a39  # static:nonroot

COPY --from=builder /gateway /gateway

USER nonroot:nonroot
# Run with: docker run --read-only --tmpfs /tmp aga2aga
# Kubernetes: securityContext.readOnlyRootFilesystem: true, allowPrivilegeEscalation: false
ENTRYPOINT ["/gateway"]
