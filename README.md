# DO NOT USE THIS PROJECT. I'M ONLY PLAYING AROUND (FOR NOW)

```bash
go get -d -v -t

go test ./... -cover -run UnitTest

# env: SCRAPE_INTERVAL

docker run --rm \
    -v $PWD:/usr/src/myapp \
    -w /usr/src/myapp \
    -v go:/go golang:1.6 \
    bash -c "go get -d -v -t && CGO_ENABLED=0 GOOS=linux go build -v -o docker-flow-monitor"

docker image build -t vfarcic/docker-flow-monitor:beta .

docker image push vfarcic/docker-flow-monitor:beta

docker service create --name monitor \
    -p 8080:8080 \
    -p 9090:9090 \
    vfarcic/docker-flow-monitor:beta

docker service ps monitor

curl "http://localhost:8080/v1/docker-flow-monitor/alert?alertName=my-alert&alertIf=my-if&alertFrom=my-from" | jq '.'

docker service logs monitor

open "http://localhost:9090"
```