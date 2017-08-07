#!/usr/bin/env bash

./scripts/dm-swarm.sh

eval $(docker-machine env swarm-1)

docker network create -d overlay proxy

docker stack deploy \
    -c stacks/docker-flow-proxy-mem.yml \
    proxy

docker network create -d overlay monitor

echo "route:
  group_by: [service]
  repeat_interval: 1h
  receiver: 'slack'

receivers:
  - name: 'slack'
    slack_configs:
      - send_resolved: true
        title: '[{{ .Status | toUpper }}] {{ .GroupLabels.service }} service is in danger!'
        title_link: 'http://$(docker-machine ip swarm-1)/monitor/alerts'
        text: '{{ .CommonAnnotations.summary}}'
        api_url: 'https://hooks.slack.com/services/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'
" | docker secret create alert_manager_config -

DOMAIN=$(docker-machine ip swarm-1) \
    docker stack deploy \
    -c stacks/docker-flow-monitor-slack.yml \
    monitor

echo "admin" | \
    docker secret create jenkins-user -

echo "admin" | \
    docker secret create jenkins-pass -

export SLACK_IP=$(ping \
    -c 1 devops20.slack.com \
    | awk -F'[()]' '/PING/{print $2}')

docker stack deploy \
    -c stacks/jenkins-scale.yml jenkins