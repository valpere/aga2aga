# Stage 1: Build
FROM golang:1.24-bookworm AS builder

WORKDIR /src

# Copy dependency manifests first for layer caching.
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build a static binary.
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /gateway ./cmd/gateway

# Stage 2: Runtime — distroless contains only the binary, no shell or toolchain.
FROM gcr.io/distroless/static:nonroot

COPY --from=builder /gateway /gateway

USER nonroot:nonroot
ENTRYPOINT ["/gateway"]
