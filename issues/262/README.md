```bash
docker network create --driver overlay backend

docker service create --name swarm-listener \
    --network backend \
    --mount "type=bind,source=/var/run/docker.sock,target=/var/run/docker.sock" \
    -e DF_NOTIFY_CREATE_SERVICE_URL=http://haproxy:8080/v1/docker-flow-proxy/reconfigure \
    -e DF_NOTIFY_REMOVE_SERVICE_URL=http://haproxy:8080/v1/docker-flow-proxy/remove \
    vfarcic/docker-flow-swarm-listener

docker service create --name haproxy \
    -p 80:80 \
    --network backend \
    -e MODE=swarm \
    -e LISTENER_ADDRESS=swarm-listener \
    -e DEBUG=true \
    -e STATS_USER=stats \
    -e STATS_PASS=stats \
    vfarcic/docker-flow-proxy

# Enter the swarm-listener container

apk add --update curl

curl -v http://haproxy:8080/v1/docker-flow-proxy/config

# Enter the proxy container

apk add --update curl

curl -v http://haproxy:8080/v1/docker-flow-proxy/reconfigure?distribute=true\&port=80\&serviceName=mywebsite\&servicePath=%2F
```