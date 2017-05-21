#!/usr/bin/env bash

./scripts/dm-swarm.sh

eval $(docker-machine env swarm-1)

docker network create -d overlay proxy

docker stack deploy \
    -c stacks/docker-flow-proxy-mem.yml \
    proxy

docker network create -d overlay monitor

DOMAIN=$(docker-machine ip swarm-1) \
    docker stack deploy \
    -c stacks/docker-flow-monitor-mem.yml \
    monitor

docker stack deploy \
    -c stacks/exporters-mem.yml \
    exporter

docker stack deploy \
    -c stacks/go-demo-mem.yml \
    go-demo
