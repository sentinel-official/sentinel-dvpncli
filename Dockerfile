# Build Stage
FROM golang:1.25-alpine3.22 AS build

# Copy the files
COPY . /root/

# Install dependencies and build in one step, also leverage build cache
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    apk add --no-cache git make && \
    cd /root/ && make --jobs=$(nproc) install

# Final Stage
FROM alpine:3.22

# Install necessary packages in a single layer
RUN apk add --no-cache \
    iptables \
    openvpn \
    v2ray \
    wireguard-tools && \
    rm -rf /etc/v2ray/ /usr/share/v2ray/

# Copy the built binary from the build stage
COPY --from=build /go/bin/sentinel-dvpncli /usr/local/bin/dvpncli

# Set the entrypoint for the container
ENTRYPOINT ["dvpncli"]
