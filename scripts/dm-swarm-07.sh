#!/usr/bin/env bash

./scripts/dm-swarm.sh

eval $(docker-machine env swarm-1)

docker network create -d overlay proxy

docker stack deploy \
    -c stacks/docker-flow-proxy-mem.yml \
    proxy

docker network create -d overlay monitor

echo 'route:
  receiver: "slack"
  repeat_interval: 1h

receivers:
  - name: "slack"
    slack_configs:
      - send_resolved: true
        title: "This is a title {{ .container_label_com_docker_swarm_service_name}}"
        text: "Something horrible happened! Run for your lives!"
        api_url: "https://hooks.slack.com/services/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"
' | docker secret create alert_manager_config -

DOMAIN=$(docker-machine ip swarm-1) \
    docker stack deploy \
    -c stacks/docker-flow-monitor-slack.yml \
    monitor

docker stack deploy \
    -c stacks/exporters-alert.yml \
    exporter

docker stack deploy \
    -c stacks/go-demo-alert.yml \
    go-demo
