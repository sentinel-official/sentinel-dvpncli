FROM golang:1.25-alpine3.22 AS build

COPY . /root/

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    apk add git make && \
    cd /root/ && make --jobs=$(nproc) install

FROM alpine:3.22

COPY --from=build /go/bin/sentinel-dvpncli /usr/local/bin/dvpncli

RUN apk add --no-cache iptables openvpn v2ray wireguard-tools && \
    rm -rf /etc/v2ray/ /usr/share/v2ray/

ENTRYPOINT ["dvpncli"]
