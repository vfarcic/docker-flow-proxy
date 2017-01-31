FROM haproxy:1.7-alpine
MAINTAINER 	Viktor Farcic <viktor@farcic.com>

RUN apk add --no-cache --virtual .build-deps curl unzip && \
    curl -SL https://releases.hashicorp.com/consul-template/0.13.0/consul-template_0.13.0_linux_amd64.zip -o /usr/local/bin/consul-template.zip && \
    unzip /usr/local/bin/consul-template.zip -d /usr/local/bin/ && \
    rm -f /usr/local/bin/consul-template.zip && \
    chmod +x /usr/local/bin/consul-template && \
    apk del .build-deps

RUN mkdir /lib64 && ln -s /lib/libc.musl-x86_64.so.1 /lib64/ld-linux-x86-64.so.2
RUN mkdir -p /cfg/tmpl
RUN mkdir /consul_templates
RUN mkdir /templates
RUN mkdir /certs

ENV CONNECTION_MODE="http-server-close" \
    CONSUL_ADDRESS="" \
    DEBUG="false" \
    LISTENER_ADDRESS="" \
    MODE="default" \
    PROXY_INSTANCE_NAME="docker-flow" \
    SERVICE_NAME="proxy" \
    STATS_USER="admin" STATS_PASS="admin" \
    TIMEOUT_HTTP_REQUEST="5" TIMEOUT_HTTP_KEEP_ALIVE="15" TIMEOUT_CLIENT="20" TIMEOUT_CONNECT="5" TIMEOUT_QUEUE="30" TIMEOUT_SERVER="20" TIMEOUT_TUNNEL="3600" \
    USERS="" \
    EXTRA_FRONTEND=""

EXPOSE 80
EXPOSE 443
EXPOSE 8080

CMD ["docker-flow-proxy", "server"]

COPY errorfiles /errorfiles
COPY haproxy.cfg /cfg/haproxy.cfg
COPY haproxy.tmpl /cfg/tmpl/haproxy.tmpl
COPY docker-flow-proxy /usr/local/bin/docker-flow-proxy
RUN chmod +x /usr/local/bin/docker-flow-proxy
