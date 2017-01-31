```bash
docker network create --driver overlay proxy

docker network create --driver overlay go-demo

docker service create --name go-demo-db \
    --network go-demo \
    mongo

docker service create --name go-demo \
    -e DB=go-demo-db \
    --network go-demo \
    --label com.df.notify=true \
    --label com.df.distribute=true \
    --label com.df.servicePath=/demo \
    --label com.df.port=8080 \
    vfarcic/go-demo








docker run --rm -v $PWD:/usr/src/myapp -w /usr/src/myapp -v go:/go golang:1.6 bash -c "go get -d -v -t && CGO_ENABLED=0 GOOS=linux go build -v -o docker-flow-swarm-listener"

docker build -t vfarcic/docker-flow-swarm-listener:test .

docker push vfarcic/docker-flow-swarm-listener:test

docker service create --name swarm-listener \
    --network proxy \
    --mount "type=bind,source=/var/run/docker.sock,target=/var/run/docker.sock" \
    -e DF_NOTIFY_CREATE_SERVICE_URL=http://proxy:8080/v1/docker-flow-proxy/reconfigure \
    -e DF_NOTIFY_REMOVE_SERVICE_URL=http://proxy:8080/v1/docker-flow-proxy/remove \
    -e DF_RETRY=2 \
    --constraint 'node.role==manager' \
    vfarcic/docker-flow-swarm-listener:test

docker service create --name proxy \
    -p 80:80 \
    -p 443:443 \
    --network proxy \
    -e MODE=swarm \
    -e LISTENER_ADDRESS=swarm-listener \
    vfarcic/docker-flow-proxy

docker ps

docker service rm proxy swarm-listener
```