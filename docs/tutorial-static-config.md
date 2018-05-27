```bash
git clone https://github.com/docker-flow/docker-flow-monitor.git

cd docker-flow-monitor

./scripts/dm-swarm.sh

eval $(docker-machine env swarm-1)

docker network create -d overlay monitor

docker stack deploy \
    -c stacks/docker-flow-monitor-tutorial.yml \
    monitor

echo '
  - job_name: "mongo-instance"
    scrape_interval: 5s
    static_configs:
      - targets: ["1.2.3.4:9100"]
' | docker secret create scrape_mongo -

docker service update --secret-add scrape_mongo monitor_monitor

open "http://$(docker-machine ip swarm-1):9090/config"
```

[screen output]
```
global:
  scrape_interval: 10s

scrape_configs:
  - job_name: "mongo-instance"
    scrape_interval: 5s
    static_configs:
      - targets: ["1.2.3.4:9100"]
```

```bash
docker stack deploy \
    -c stacks/exporters-tutorial.yml \
    exporter

open "http://$(docker-machine ip swarm-1):9090/config"
```

[screen output]
```
global:
  scrape_interval: 10s

scrape_configs:
  - job_name: "exporter_cadvisor"
    dns_sd_configs:
      - names: ["tasks.exporter_cadvisor"]
        type: A
        port: 8080
  - job_name: "exporter_node-exporter"
    dns_sd_configs:
      - names: ["tasks.exporter_node-exporter"]
        type: A
        port: 9100

  - job_name: "mongo-instance"
    scrape_interval: 5s
    static_configs:
      - targets: ["1.2.3.4:9100"]


rule_files:
  - 'alert.rules'
```

```bash
docker-machine rm -f swarm-1 swarm-2 swarm-3
```
