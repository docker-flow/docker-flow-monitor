# DO NOT USE THIS PROJECT. I'M ONLY PLAYING AROUND (FOR NOW)

## TODO

* Change /prom to /monitor
* Create stacks
* Write a blog port
* Add to the book
* Add to [http://training.play-with-docker.com/](http://training.play-with-docker.com/)

## Build

```bash
go get -d -v -t

go test ./... -cover -run UnitTest

docker run --rm \
    -v $PWD:/usr/src/myapp \
    -w /usr/src/myapp \
    -v go:/go golang:1.6 \
    bash -c "go get -d -v -t && CGO_ENABLED=0 GOOS=linux go build -v -o docker-flow-monitor"

docker image build -t vfarcic/docker-flow-monitor .

docker image push vfarcic/docker-flow-monitor
```

## Setup

```bash
docker service create --name monitor \
    -p 9090:9090 \
    prom/prometheus

docker service ps monitor

# proxy_url

open "http://localhost:9090"

open "http://localhost:9090/config"

docker service rm monitor

docker network create -d overlay monitor

docker network create -d overlay proxy

docker service create --name monitor \
    -p 9090:9090 \
    --network monitor \
    -e GLOBAL_SCRAPE_INTERVAL=5s \
    vfarcic/docker-flow-monitor

docker service ps monitor

open "http://localhost:9090/config"

docker service rm monitor
```

## Integration With DFP

```bash
docker service create --name swarm-listener \
    --network monitor --network proxy \
    --mount "type=bind,source=/var/run/docker.sock,target=/var/run/docker.sock" \
    -e DF_NOTIFY_CREATE_SERVICE_URL=http://proxy:8080/v1/docker-flow-proxy/reconfigure \
    -e DF_NOTIFY_REMOVE_SERVICE_URL=http://proxy:8080/v1/docker-flow-proxy/remove \
    --constraint 'node.role==manager' \
    vfarcic/docker-flow-swarm-listener

docker service create --name proxy \
    -p 80:80 -p 443:443 \
    --network proxy --network monitor \
    --env LISTENER_ADDRESS=swarm-listener \
    --env MODE=swarm \
    --env STATS_USER=admin \
    --env STATS_PASS=admin \
    vfarcic/docker-flow-proxy

docker service create --name monitor \
    --network monitor \
    --env GLOBAL_SCRAPE_INTERVAL=5s \
    --env ARG_WEB_ROUTE-PREFIX=/prom \
    --env ARG_WEB_EXTERNAL-URL=http://localhost/prom \
    --label com.df.notify=true \
    --label com.df.distribute=true \
    --label com.df.servicePath=/prom \
    --label com.df.port=9090 \
    vfarcic/docker-flow-monitor

docker service ps monitor

open "http://localhost/prom"
```

## Exporters

```bash
docker service update \
    --env-add DF_NOTIFY_CREATE_SERVICE_URL=http://monitor:8080/v1/docker-flow-monitor/reconfigure,http://proxy:8080/v1/docker-flow-proxy/reconfigure \
    swarm-listener

docker service create --name haproxy-exporter \
    --network monitor --network proxy \
    --label com.df.notify=true \
    --label com.df.scrapePort=9101 \
    quay.io/prometheus/haproxy-exporter \
    -haproxy.scrape-uri="http://admin:admin@proxy/admin?stats;csv"

docker service ps haproxy-exporter

open "http://localhost/prom/config"

docker network create -d overlay go-demo

docker service create --name go-demo-db \
  --network go-demo \
  mongo

docker service create --name go-demo \
    -e DB=go-demo-db \
    --network go-demo --network proxy \
    --label com.df.notify=true \
    --label com.df.distribute=true \
    --label com.df.servicePath=/demo \
    --label com.df.port=8080 \
    vfarcic/go-demo

docker service ps go-demo

curl -i "http://localhost/demo/hello"

open "http://localhost/prom/graph"

# http_request_duration_microseconds

for ((n=0;n<200;n++)); do
    curl "http://localhost/demo/hello"
done

open "http://localhost/prom/graph"

# http_request_duration_microseconds

docker service create --name cadvisor \
    --mode global \
    --network monitor \
    --mount "type=bind,source=/,target=/rootfs" \
    --mount "type=bind,source=/var/run,target=/var/run" \
    --mount "type=bind,source=/sys,target=/sys" \
    --mount "type=bind,source=/var/lib/docker,target=/var/lib/docker" \
    --label com.df.notify=true \
    --label com.df.scrapePort=8080 \
    google/cadvisor

docker service ps cadvisor

docker service logs swarm-listener

docker service logs monitor

open "http://localhost/prom/config"

open "http://localhost/prom/graph"

# container_memory_usage_bytes{container_label_com_docker_swarm_service_name="go-demo"}

for ((n=0;n<200;n++)); do
    curl "http://localhost/demo/hello"
done

# container_memory_usage_bytes{container_label_com_docker_swarm_service_name="go-demo"}
```

## Custom Exporters

TODO: Explanation

## Alerts

```bash
docker service update \
    --label-add com.df.alertName=mem \
    --label-add com.df.alertIf='container_memory_usage_bytes{container_label_com_docker_swarm_service_name="go-demo"} > 10000000' \
    go-demo

open "http://localhost/prom/config"

open "http://localhost/prom/alerts"

open "http://localhost/prom/graph"

# container_memory_usage_bytes{container_label_com_docker_swarm_service_name="go-demo"}

docker service update \
    --label-add com.df.alertName=mem \
    --label-add com.df.alertIf='container_memory_usage_bytes{container_label_com_docker_swarm_service_name="go-demo"} > 1000000' \
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

# TODO: Alert manager

# TODO: Failover

docker service update \
    --env-add LISTENER_ADDRESS=swarm-listener \
    monitor
```

# TODO: Everything at once through stacks
```
