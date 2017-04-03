# DO NOT USE THIS PROJECT. I'M ONLY PLAYING AROUND (FOR NOW)

## TODO

* Test in bash
* Remove beta tags
* Create stacks

## Build

```bash
go get -d -v -t

go test ./... -cover -run UnitTest

docker run --rm \
    -v $PWD:/usr/src/myapp \
    -w /usr/src/myapp \
    -v go:/go golang:1.6 \
    bash -c "go get -d -v -t && CGO_ENABLED=0 GOOS=linux go build -v -o docker-flow-monitor"

docker image build -t vfarcic/docker-flow-monitor:beta .

docker image push vfarcic/docker-flow-monitor:beta
```

## Setup

```bash
docker service create --name monitor \
    -p 9090:9090 \
    prom/prometheus

docker service ps monitor

open "http://localhost:9090"

open "http://localhost:9090/config"

docker service rm monitor

docker network create -d overlay monitor

docker network create -d overlay proxy

# TODO: Add to the proxy

docker service create --name monitor \
    -p 9090:9090 \
    --network monitor \
    -e SCRAPE_INTERVAL=5 \
    -e LISTENER_ADDRESS=swarm-listener \
    vfarcic/docker-flow-monitor:beta

docker service ps monitor

open "http://localhost:9090/config"

docker service create --name proxy \
    -p 80:80 -p 443:443 \
    --network proxy --network monitor \
    -e LISTENER_ADDRESS=swarm-listener \
    -e MODE=swarm \
    -e STATS_USER=admin \
    -e STATS_PASS=admin \
    vfarcic/docker-flow-proxy

docker service ps proxy

docker service create --name swarm-listener \
    --network monitor --network proxy \
    --mount "type=bind,source=/var/run/docker.sock,target=/var/run/docker.sock" \
    -e DF_NOTIFY_CREATE_SERVICE_URL=http://monitor:8080/v1/docker-flow-monitor/reconfigure,http://proxy:8080/v1/docker-flow-proxy/reconfigure \
    -e DF_NOTIFY_REMOVE_SERVICE_URL=http://proxy:8080/v1/docker-flow-proxy/remove \
    --constraint 'node.role==manager' \
    vfarcic/docker-flow-swarm-listener

docker service ps swarm-listener
```

## Exporters

```bash
# TODO: Add the exporter manally

docker service create --name haproxy-exporter \
    --network monitor --network proxy \
    --label com.df.notify=true \
    --label com.df.scrapePort=9101 \
    quay.io/prometheus/haproxy-exporter \
    -haproxy.scrape-uri="http://admin:admin@proxy?stats;csv"

open "http://localhost:9090/config"

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

open "http://localhost:9090/graph"

for ((n=0;n<20;n++)); do
    curl "http://localhost/demo/hello"
done

open "http://localhost:9090/graph"

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

open "http://localhost:9090/config"

open "http://localhost:9090/graph"

for ((n=0;n<20;n++)); do
    curl "http://localhost/demo/hello"
done
```

## Custom Exporters

TODO: Explanation

## Alerts

```bash
docker service update \
    --limit-memory 20mb \
    --reserve-memory 10mb \
    go-demo

open "http://localhost:9090/graph"

# container_spec_memory_limit_bytes{container_label_com_docker_swarm_service_name="go-demo"}
# container_memory_usage_bytes{container_label_com_docker_swarm_service_name="go-demo"}

docker service update \
    --label-add com.df.alertName=memlimit \
    --label-add com.df.alertIf='container_memory_usage_bytes{container_label_com_docker_swarm_service_name="go-demo"} < (container_spec_memory_limit_bytes{container_label_com_docker_swarm_service_name="go-demo"} * 0.8)' \
    go-demo

curl "http://localhost:8080/v1/docker-flow-monitor?alertName=godemoalert&alertIf=container_memory_usage_bytes{container_label_com_docker_swarm_service_name="go-demo"} < (container_spec_memory_limit_bytes{container_label_com_docker_swarm_service_name="go-demo"} * 0.8)" | jq '.'

# TODO: Add alert-from example

# TODO: Alert manager

# TODO: Failover

# TODO: Everything at once through stacks
```
