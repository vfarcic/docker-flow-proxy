```bash
docker network create --driver overlay proxy

# Publish the port 8888 so that the service can be tested without the proxy
docker service create --network proxy --name ztest -p 8888:8888 paulhkhsu/ztest

# Test that the service is indeed reachable through path `/ztest`
curl localhost:8888/ztest

# The output is: `{"timestamp":1495667406795,"status":404,"error":"Not Found","message":"No message available","path":"/ztest"}`

# Check whether the service is accessible through root path
curl localhost:8888

# The output is `Greetings from Spring Boot!`

# Remove the port of the service. We don't need it any more since we'll reconfigure the proxy
docker service update --publish-rm 8888 ztest

# Create the proxy service
docker service create --name proxy -p 80:80 -p 443:443 -p 8080:8080 --network proxy -e MODE=swarm vfarcic/docker-flow-proxy

# Reconfigure the proxy with the root path instead
curl "localhost:8080/v1/docker-flow-proxy/reconfigure?serviceName=ztest&servicePath=/&port=8888"

# Confirm that it works
curl localhost

# Reconfigure the proxy with the root path instead
curl "localhost:8080/v1/docker-flow-proxy/reconfigure?serviceName=ztest&servicePath=/ztest&port=8888&reqPathSearch=/ztest&reqPathReplace=/"

# Confirm that it works
curl localhost/ztest
```

```bash
docker network create --driver overlay proxy

docker service create \
    --network proxy \
    --name ztest \
    paulhkhsu/ztest

docker service create \
    --network proxy \
    --name ztest1 \
    paulhkhsu/ztest

docker service create \
    --name proxy \
    -p 80:80 -p 443:443 -p 8080:8080 \
    --network proxy \
    -e MODE=swarm \
    vfarcic/docker-flow-proxy

curl "localhost:8080/v1/docker-flow-proxy/reconfigure?serviceName=ztest&servicePath=/ztest&port=8888&reqPathSearch=/ztest&reqPathReplace=/"

curl localhost/ztest

curl "localhost:8080/v1/docker-flow-proxy/reconfigure?serviceName=ztest1&servicePath=/ztest1&port=8888&reqPathSearch=/ztest1&reqPathReplace=/&aclName=01-ztest1"

curl localhost/ztest1
```