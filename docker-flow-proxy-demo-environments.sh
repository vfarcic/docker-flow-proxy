#!/usr/bin/env bash

set -e


proxyStatusMachine=`docker-machine status proxy`

if [[ $proxyStatusMachine = "Running" ]]; then
  echo Proxy Machine Already Running.
elif [[ $proxyStatusMachine == "Stopped" ]]; then
  docker-machine start proxy
else
  docker-machine create -d virtualbox proxy
fi

export DOCKER_IP=$(docker-machine ip proxy)

export CONSUL_IP=$(docker-machine ip proxy)

eval "$(docker-machine env proxy)"

docker-compose up -d consul proxy

docker ps -a

swarmMasterStatusMachine=`docker-machine status swarm-master`

if [[ $swarmMasterStatusMachine = "Running" ]]; then
  echo Swarm Master Machine Already Running.
elif [[ $swarmMasterStatusMachine == "Stopped" ]]; then
  docker-machine start swarm-master
else
  docker-machine create -d virtualbox \
    --swarm --swarm-master \
    --swarm-discovery="consul://$CONSUL_IP:8500" \
    --engine-opt="cluster-store=consul://$CONSUL_IP:8500" \
    --engine-opt="cluster-advertise=eth1:2376" \
    swarm-master
fi

swarmNode1StatusMachine=`docker-machine status swarm-node-1`

if [[ $swarmNode1StatusMachine = "Running" ]]; then
  echo Swarm Node 1 Machine Already Running.
elif [[ $swarmNode1StatusMachine == "Stopped" ]]; then
  docker-machine start swarm-node-1
else
  docker-machine create -d virtualbox \
    --swarm \
    --swarm-discovery="consul://$CONSUL_IP:8500" \
    --engine-opt="cluster-store=consul://$CONSUL_IP:8500" \
    --engine-opt="cluster-advertise=eth1:2376" \
    swarm-node-1
fi

swarmNode2StatusMachine=`docker-machine status swarm-node-2`

if [[ $swarmNode2StatusMachine = "Running" ]]; then
  echo Swarm Node 2 Machine Already Running.
elif [[ $swarmNode2StatusMachine == "Stopped" ]]; then
  docker-machine start swarm-node-2
else
  docker-machine create -d virtualbox \
    --swarm \
    --swarm-discovery="consul://$CONSUL_IP:8500" \
    --engine-opt="cluster-store=consul://$CONSUL_IP:8500" \
    --engine-opt="cluster-advertise=eth1:2376" \
    swarm-node-2
fi

eval "$(docker-machine env swarm-master)"

export DOCKER_IP=$(docker-machine ip swarm-master)

docker-compose up -d registrator

eval "$(docker-machine env swarm-node-1)"

export DOCKER_IP=$(docker-machine ip swarm-node-1)

docker-compose up -d registrator

eval "$(docker-machine env swarm-node-2)"

export DOCKER_IP=$(docker-machine ip swarm-node-2)

docker-compose up -d registrator
