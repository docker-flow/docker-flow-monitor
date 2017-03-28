# DO NOT USE THIS PROJECT. I'M ONLY PLAYING AROUND (FOR NOW)

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

docker service rm monitor

docker network create -d overlay monitor

# TODO: Remove :beta

docker service create --name monitor \
    -p 9090:9090 \
    --network monitor \
    -e SCRAPE_INTERVAL=10 \
    vfarcic/docker-flow-monitor:beta

docker service ps monitor

open "http://localhost:9090/config"

docker service create --name swarm-listener \
    --network monitor \
    --mount "type=bind,source=/var/run/docker.sock,target=/var/run/docker.sock" \
    -e DF_NOTIFY_CREATE_SERVICE_URL=http://monitor:8080/v1/docker-flow-monitor/reconfigure \
    vfarcic/docker-flow-swarm-listener

docker service ps swarm-listener

# TODO: Create a stack with monitor and swarm-listener and add it to https://github.com/vfarcic/docker-flow-stacks
```

# Scrapes

```bash
# TODO: Create a stack with exporters and add it to https://github.com/vfarcic/docker-flow-stacks

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

curl "http://swarm-listener:8080/v1/docker-flow-swarm-listener/get-services"
```

# Alerts

```bash
curl "http://localhost:8080/v1/docker-flow-monitor?alertName=my-alert&alertIf=my-if&alertFrom=my-from" | jq '.'
```

# DFP