FROM golang:1.7 AS build
ADD . /src
WORKDIR /src
RUN go get -d -v -t
RUN go test --cover ./... --run UnitTest
RUN go build -v -o docker-flow-proxy



FROM haproxy:1.7-alpine
MAINTAINER 	Viktor Farcic <viktor@farcic.com>

RUN mkdir /lib64 && ln -s /lib/libc.musl-x86_64.so.1 /lib64/ld-linux-x86-64.so.2
RUN mkdir -p /cfg/tmpl /templates /certs /logs

ENV CERTS="" \
    CAPTURE_REQUEST_HEADER="" \
    CFG_TEMPLATE_PATH="/cfg/tmpl/haproxy.tmpl" \
    CHECK_RESOLVERS=false \
    CONNECTION_MODE="http-keep-alive" \
    DEBUG="false" \
    DEFAULT_PORTS="80,443:ssl" \
    DO_NOT_RESOLVE_ADDR="false" \
    EXTRA_FRONTEND="" \
    LISTENER_ADDRESS="" \
    MODE="default" \
    PROXY_INSTANCE_NAME="docker-flow" \
    RELOAD_INTERVAL="5000" REPEAT_RELOAD=false \
    SERVICE_NAME="proxy" SERVICE_DOMAIN_ALGO="hdr(host)" \
    STATS_USER="" STATS_USER_ENV="STATS_USER" STATS_PASS="" STATS_PASS_ENV="STATS_PASS" STATS_URI="" STATS_URI_ENV="STATS_URI" \
    TIMEOUT_HTTP_REQUEST="5" TIMEOUT_HTTP_KEEP_ALIVE="15" TIMEOUT_CLIENT="20" TIMEOUT_CONNECT="5" TIMEOUT_QUEUE="30" TIMEOUT_SERVER="20" TIMEOUT_TUNNEL="3600" \
    USERS="" \
    SKIP_ADDRESS_VALIDATION="true" \
    SSL_BIND_OPTIONS="no-sslv3" SSL_BIND_CIPHERS="ECDH+AESGCM:DH+AESGCM:ECDH+AES256:DH+AES256:ECDH+AES128:DH+AES:RSA+AESGCM:RSA+AES:!aNULL:!MD5:!DSS"

EXPOSE 80
EXPOSE 443
EXPOSE 8080

CMD ["docker-flow-proxy", "server"]

COPY errorfiles /errorfiles
COPY haproxy.cfg /cfg/haproxy.cfg
COPY haproxy.tmpl /cfg/tmpl/haproxy.tmpl
COPY --from=build /src/docker-flow-proxy /usr/local/bin/docker-flow-proxy
RUN chmod +x /usr/local/bin/docker-flow-proxy
