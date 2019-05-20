#!/usr/bin/env bash

./scripts/dm-swarm-14_00.sh
if [[ $? -ne 0 ]]; then
    exit 1
fi

eval $(docker-machine env swarm-1)

docker network create -d overlay --attachable proxy
docker stack deploy -c stacks/docker-flow-proxy.yml proxy

docker network create -d overlay --attachable monitor


###########################################################################
#   PROMETHEUS MONITORING
#
#  Install "Incoming WebHooks" Slack App first, then save your web hook
#   This API will be used as a default receiver by the alert_manager
SERVICE_NAME='go-demo_main'
SLACK_API_URL='https://hooks.slack.com/services/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'
SLACK_TOKEN='DevOps22'

echo "route:
  group_by: [service,scale]
  repeat_interval: 5m
  group_interval: 5m
  receiver: 'slack'
  routes:
  - match:
      service: '${SERVICE_NAME}'
      scale: 'up'
    receiver: 'jenkins-${SERVICE_NAME}-up'
  - match:
      service: '${SERVICE_NAME}'
      scale: 'down'
    receiver: 'jenkins-${SERVICE_NAME}-down'

receivers:
  - name: 'slack'
    slack_configs:
      - send_resolved: true
        title: '[{{ .Status | toUpper }}] {{ .GroupLabels.service }} service is in danger!'
        title_link: 'http://$(docker-machine ip swarm-1)/monitor/alerts'
        text: '{{ .CommonAnnotations.summary}}'
        api_url: ${SLACK_API_URL}
  - name: 'jenkins-${SERVICE_NAME}-up'
    webhook_configs:
      - send_resolved: false
        url: 'http://$(docker-machine ip swarm-1)/jenkins/job/service-scale/buildWithParameters?token=${SLACK_TOKEN}&service=${SERVICE_NAME}&scale=1'
  - name: 'jenkins-${SERVICE_NAME}-down'
    webhook_configs:
      - send_resolved: false
        url: 'http://$(docker-machine ip swarm-1)/jenkins/job/service-scale/buildWithParameters?token=${SLACK_TOKEN}&service=${SERVICE_NAME}&scale=-1'
" | docker secret create alert_manager_config -

DOMAIN=$(docker-machine ip swarm-1)

docker stack deploy \
    -c stacks/docker-flow-monitor-slack.yml \
    monitor

echo "admin" | docker secret create jenkins-user -
echo "admin" | docker secret create jenkins-pass -

export SLACK_IP=`ping -c 1 devops20.slack.com | awk -F '[()]' '/PING/{print $2}'`

docker stack deploy -c stacks/jenkins-scale.yml jenkins
sleep 2
docker stack ps jenkins
echo
echo "Configure Jenkins jobs at: "
echo "http://$(docker-machine ip swarm-1)/jenkins/job/service-scale/configure"
echo "Prometheus alert screen at: http://$(docker-machine ip swarm-1)/monitor/alerts"
echo

docker stack deploy -c stacks/go-demo-instrument-alert-error.yml go-demo

echo "Please wait.. "
sleep 5
docker service ls

echo
echo " Web service exposed at http://${DOMAIN}/demo/hello"
echo " Please now copy conf/jenkins/service-scale.groovy in http://${DOMAIN}/jenkins/job/service-scale/ \
        configuration and check if parameters (service, scale) are accepted"
