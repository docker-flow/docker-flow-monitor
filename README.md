# DO NOT USE THIS PROJECT. I'M ONLY PLAYING AROUND (FOR NOW)

## Alert Manager

```bash
echo '
route:
  receiver: "slack"
  repeat_interval: 1h

receivers:
    - name: "slack"
      slack_configs:
          - send_resolved: true
            text: "Hello!"
            api_url: "https://hooks.slack.com/services/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"
' | tee alertmanager.yml

docker service create --name alert-manager \
    -p 9093:9093 \
    --mount "type=bind,source=$PWD/alertmanager.yml,target=/etc/alertmanager/config.yml" \
    prom/alertmanager

curl -H "Content-Type: application/json" -d '[{"labels":{"alertname":"TestAlert1"}}]' $(docker-machine ip swarm-1):9093/api/v1/alerts

docker service update \
    --publish-rm 9093:9093 \
    alert-manager

docker service update \
    --env-add ARG_ALERTMANAGER_ARL=http://alert-manager:9093
    monitor
```

## Jenkins metrics

## Stacks

## Prometheus persistence

## Microscaling