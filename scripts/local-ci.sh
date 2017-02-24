#!/bin/bash
set -e
if [[ ( -z ${DOCKER_HUB_USER} ) || ( -z ${HOST_IP} ) ]]; then
    echo "set DOCKER_HUB_USER variable to your docker hub account, HOST_IP to your host Ip before running"
    exit 1
fi

echo Running in $PWD
docker run --rm -v $PWD:/usr/src/myapp -w /usr/src/myapp -v go:/go golang:1.6 bash -c "go get -d -v -t && go test --cover ./... --run UnitTest && go build -v -o docker-flow-proxy"
docker build -t $DOCKER_HUB_USER/docker-flow-proxy .
docker-compose -f docker-compose-test.yml up -d staging-dep
docker-compose -f docker-compose-test.yml run --rm staging
docker-compose -f docker-compose-test.yml down
docker tag $DOCKER_HUB_USER/docker-flow-proxy $DOCKER_HUB_USER/docker-flow-proxy:beta
docker push $DOCKER_HUB_USER/docker-flow-proxy:beta
docker-compose -f docker-compose-test.yml run --rm staging-swarm