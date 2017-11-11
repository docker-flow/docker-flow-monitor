#!/usr/bin/env bash

./scripts/dm-swarm.sh

eval $(docker-machine env swarm-1)

docker network create -d overlay proxy

docker stack deploy \
    -c stacks/docker-flow-proxy.yml \
    proxy

docker network create -d overlay monitor

DOMAIN=$(docker-machine ip swarm-1) \
    docker stack deploy \
    -c stacks/docker-flow-monitor-proxy.yml \
    monitor




