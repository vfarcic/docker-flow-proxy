#!/bin/bash

set -e

if [[ ( -z ${DOCKER_HUB_USER} ) || ( -z ${HOST_IP} ) ]]; then
    echo "set DOCKER_HUB_USER variable to your docker hub account, HOST_IP to your host Ip before running"
    exit 1
fi

echo Running in $PWD

docker-compose \
    -f docker-compose-test.yml \
    run --rm unit

docker image build \
    -t $DOCKER_HUB_USER/docker-flow-proxy:beta \
    .

docker image push \
    $DOCKER_HUB_USER/docker-flow-proxy:beta

docker-compose \
    -f docker-compose-test.yml \
    run --rm staging-swarm