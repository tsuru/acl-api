FROM golang:1.20-alpine3.18 AS builder
COPY . /go/src/github.com/tsuru/acl-api
WORKDIR /go/src/github.com/tsuru/acl-api
RUN set -x \
    && apk add --update gcc git make musl-dev \
    && make build

FROM alpine:3.18
COPY --from=builder /go/src/github.com/tsuru/acl-api/bin/acl-api /bin/acl-api
ARG gke_auth_plugin_version=0.1.1
ARG TARGETARCH
RUN set -x \
    && apk add --update --no-cache curl ca-certificates \
    && curl -fsSL "https://github.com/traviswt/gke-auth-plugin/releases/download/${gke_auth_plugin_version}/gke-auth-plugin_Linux_$( [[ ${TARGETARCH} == 'amd64' ]] && echo 'x86_64' || echo ${TARGETARCH} ).tar.gz" \
    |  tar -C /usr/local/bin -xzvf- gke-auth-plugin \
    && gke-auth-plugin version
CMD ["/bin/acl-api"]
