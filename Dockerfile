# Build stage
FROM golang:1.26-alpine3.23 AS build

# Set working directory
WORKDIR /root

# Install build dependencies
RUN apk add --no-cache \
    git \
    make

# Cache Go modules
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Copy source into the working directory
COPY . .

# Build sentinel-dvpncli
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    make --jobs=$(nproc) install

# Runtime stage
FROM alpine:3.24

# Install runtime dependencies
RUN apk add --no-cache \
    iptables \
    openresolv \
    openvpn \
    v2ray \
    wireguard-tools && \
    rm -rf /etc/v2ray/ /usr/share/v2ray/

# Copy the built binaries from build stage
COPY --from=build /go/bin/sentinel-dvpncli /usr/local/bin/dvpncli

ENTRYPOINT ["dvpncli"]
