```bash
# Add svctest.local and a.svctest.local to /etc/hosts

docker network create -d overlay proxy

docker stack deploy -c proxy.yml proxy

docker stack deploy -c go-demo.yml go-demo

curl -i "http://localhost:8080/v1/docker-flow-proxy/config"

curl -i "http://localhost:8080/v1/docker-flow-proxy/reload"

curl -i "http://svctest.local/demo/hello" # Redirects to HTTPS

curl -i "http://a.svctest.local/demo/hello" # Does NOT redirect to HTTPS
```
