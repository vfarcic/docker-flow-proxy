```bash
docker image build -t vfarcic/docker-flow-proxy:beta .

docker image push vfarcic/docker-flow-proxy:beta

docker stack deploy -c stack.yml test

curl "http://localhost:8080/v1/docker-flow-proxy/config"

curl "http://localhost/demo/hello"
```
