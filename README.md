# DO NOT USE THIS PROJECT. I'M ONLY PLAYING AROUND (FOR NOW)

## TODO

* Publish docs
* Publish a blog post
* Add to the book
* Add to [http://training.play-with-docker.com/](http://training.play-with-docker.com/)

## Setup

## Custom Exporters

TODO: Explanation

## Alerts

```bash
docker service update \
    --label-add com.df.alertName=mem \
    --label-add com.df.alertIf='container_memory_usage_bytes{container_label_com_docker_swarm_service_name="go-demo"} < 10000000' \
    go-demo

open "http://localhost/prom/config"

open "http://localhost/prom/rules"

open "http://localhost/prom/alerts"

open "http://localhost/prom/graph"

# container_memory_usage_bytes{container_label_com_docker_swarm_service_name="go-demo"}

docker service update \
    --label-add com.df.alertName=mem \
    --label-add com.df.alertIf='container_memory_usage_bytes{container_label_com_docker_swarm_service_name="go-demo"} < 1000000' \
    go-demo

open "http://localhost/prom/alerts"

docker service update \
    --limit-memory 20mb \
    --reserve-memory 10mb \
    go-demo

open "http://localhost/prom/graph"

# container_spec_memory_limit_bytes{container_label_com_docker_swarm_service_name="go-demo"}
# container_memory_usage_bytes{container_label_com_docker_swarm_service_name="go-demo"}

docker service update \
    --label-add com.df.alertName=memlimit \
    --label-add com.df.alertIf='container_memory_usage_bytes{container_label_com_docker_swarm_service_name="go-demo"}/container_spec_memory_limit_bytes{container_label_com_docker_swarm_service_name="go-demo"} > 0.1' \
    go-demo

open "http://localhost/prom/alerts"

docker service update \
    --label-add com.df.alertName=memlimit \
    --label-add com.df.alertIf='container_memory_usage_bytes{container_label_com_docker_swarm_service_name="go-demo"}/container_spec_memory_limit_bytes{container_label_com_docker_swarm_service_name="go-demo"} > 0.8' \
    go-demo

open "http://localhost/prom/alerts"

docker service update \
    --label-add com.df.alertName=memlimit \
    --label-add com.df.alertIf='container_memory_usage_bytes{container_label_com_docker_swarm_service_name="go-demo"}/container_spec_memory_limit_bytes{container_label_com_docker_swarm_service_name="go-demo"} > 0.8' \
    --label-add com.df.alertFor='1m' \
    go-demo

open "http://localhost/prom/alerts"
```

## Multiple Alerts

```bash
docker service create \
    --name node-exporter \
    --mode global \
    --network monitor \
    --mount "type=bind,source=/proc,target=/host/proc" \
    --mount "type=bind,source=/sys,target=/host/sys" \
    --mount "type=bind,source=/,target=/rootfs" \
    --mount "type=bind,source=/etc/hostname,target=/etc/host_hostname" \
    -e HOST_HOSTNAME=/etc/host_hostname \
    --label com.df.notify=true \
    --label com.df.scrapePort=9100 \
    --label com.df.alertName.1=memload \
    --label com.df.alertIf.1='(sum(node_memory_MemTotal) - sum(node_memory_MemFree + node_memory_Buffers + node_memory_Cached) ) / sum(node_memory_MemTotal) > 0.8' \
    --label com.df.alertName.2=diskload \
    --label com.df.alertIf.2='(node_filesystem_size{fstype="aufs"} - node_filesystem_free{fstype="aufs"}) / node_filesystem_size{fstype="aufs"} > 0.8' \
    basi/node-exporter:v1.13.0 \
    -collector.procfs /host/proc \
    -collector.sysfs /host/proc \
    -collector.filesystem.ignored-mount-points "^/(sys|proc|dev|host|etc)($|/)" \
    -collector.textfile.directory /etc/node-exporter/ \
    -collectors.enabled="conntrack,diskstats,entropy,filefd,filesystem,loadavg,mdadm,meminfo,netdev,netstat,stat,textfile,time,vmstat,ipvs"
```

## Removing Alerts and Scrapes

```bash
docker service update \
    --env-add DF_NOTIFY_REMOVE_SERVICE_URL=http://monitor:8080/v1/docker-flow-monitor/remove,http://proxy:8080/v1/docker-flow-proxy/remove \
    swarm-listener

docker service rm node-exporter

open "http://localhost/prom/config"

open "http://localhost/prom/rules"
```

## Failover

```bash
docker service update \
    --env-add LISTENER_ADDRESS=swarm-listener \
    monitor
```

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

curl -H "Content-Type: application/json" -d '[{"labels":{"alertname":"TestAlert1"}}]' localhost:9093/api/v1/alerts

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