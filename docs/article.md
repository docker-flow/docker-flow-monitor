## Are We Ready For Microservices?

Microservices, microservices, microservices. We are all in the process of rewriting or planning to rewrite our monoliths into microservices. Some already did it. We are putting them into containers and deploying them through one of the schedulers. We are marching into a glorious future. There's nothing that can stop us now. Except... We, as an industry, are not yet ready for microservices. One thing is to design our services in a way that they are stateless, fault tolerant, scalable, and so on.

Unless you just started a new project, chances are that you still did not reach that point and that there are quite a few legacy services floating around. However, for the sake of brevity and the urge to get to the point I'm trying to make, I will assume that all the services you're in control of are truly microservices. Does that mean that the whole system reached that nirvana state? Is deployment of a service (no matter who wrote it) fully independent from the rest of the system? Most likelly it isn't.

Let's say that you just finished the first release of your new service. Since you are practicing continuous deployment, that first release is actually the first commit to your code repository. Your CD tool of choice detected that change, and started the process. At the end of it, the service is deployed to production. I can see a smile on your face. It's that expression of happiness that can be seen only after a child is born or a service is deployed to production for the first time. That smile should not be long lasting since deploying a service is only the beginning. It needs to be integrated with the rest of the system. The proxy needs to be reconfigured. Logs parser needs to be updated with the format produced by the new service. Monitoring system needs to become aware of the new service. Alerts need to be created with the goal of sending warning and error messages when the state of the service reaches certain thresholds. The whole system needs to adapt to the new service and incorporate the new variables introduced with the commit we made a few moments ago.

How to we adapt the system so that it takes the new service into account? How do we make that service be the integral part of the system?

Unless you are writing everything yourself (in which case you must be Google), your system consists of a mixture of services written by you and services written and maintained by others. You probably use a third party proxy (hopefully that's [Docker Flow Proxy](http://proxy.dockerflow.com/)). You might have chosen the ELK stack for centralized logging. How about monitoring? It could be Prometheus. No matter the choices you made, you are not in control of the architecture of the whole system. Heck, you're probably not even in control of all the services your wrote.

TODO: Diagram

Most of the third party services are not designed to work in a highly dynamic cluster. When you deployed that first release of the service, you might have had to configure the proxy manually. You might have had to add a few parsing rules to your LogStash config. Your Prometheus targets had to be updated. New alerting rules had to be added. And so on, and so forth. Even if all those tasks are automated, the CD pipeline would have to become too big and the process would be too flaky.

> Most third-party services were designed in an era when clusters were a collection of static servers. Only a handful of those were designed to work well with containers and even fewer were adapted to work with schedulers (e.g. Swarm, k8s, Mesos/Marathon).

One of the major limitations of those third-party services is their reliance on static configuration. Take Prometheus as an example which can be extended to many other services. Every time we want to add a new target, we need to modify its configuration file and reload it. That means that we have to store that configuration file in a network drive, have some templating mechanism which updates it with every new service and, potentially, with every update of an existing service. So, we would deploy our fancy new service, update the template that generates Prometheus config, create a new config, overwrite the one stored on the network drive, and reload Prometheus. All that could be avoided if Prometheus would be configurable through its API. Still, a more extensive (not to say better) API would remove the need for templates but would NOT eliminate the need for a network drive. Its configuration is its state and it has to be preserved.

The service itself should contain all the info that describes it. If it should reconfigure a proxy, that info should be part of the service. It should contain a pattern used to output logs. It should have the addresses of targets monitoring tool should scrape from. It should have the info that will be used to launch alerts. In other words, everything that a service needs should be defined in that service. Not somewhere else. The origin of the data we need to adapt a system to the new service should not be distributed across multiple locations but inside the service we're deploying. Since we are all using containers (aren't we?), the best place to define all that info are service labels.

If your service should be accessible on path `/v1/my-fancy-service`, define a label by using argument `--label servicePath=/v1/my-fancy-service`. If Prometheus should scrape metrics on port `8080`, define a label `--label scrapePort=8080`. And so on and so forth.

Why is all this significant? Among other reasons, when we define all the data a service needs inside that service, we have a single place that defines the complete truth about a service. That makes configuration easier, it makes a team in charge of a service self-sufficient, it makes deployment easier and less error prone, and so on and so forth.

Defining all the info of a service we're developing inside that same service is not a problem. The problem is that the third-party services we're using are not designed to leverage that info. Remember, the data about a service needs to be distributed across the cluster, across all the other services that work in conjunction with the services we're developing and deploying. We do not want to define that info in multiple locations since that increases maintenance cost and introduces potential problems caused by human errors.

We do not want to define and maintain the same info in multiple locations, and we do want to keep that info at the source, but the third-party services are incapable of obtaining that data from the source. If we discard the option of modifying those third-party services, the only option left is to extend them so that they can pull or receive the data they need.

What we truly need are third-party services capable of discovering info from services we are deploying. That discovery can be pull (a service pulls info from another service) or push based (a service acts as a middle-men and pushes data from one service to another). No matter whether discovery is based on push or pull, a service that receives data needs to be able to reconfigure itself. All that needs to be combined with a system that will be able to detect that a service was deployed or updated and notify all interested parties.

This is where [Docker Flow Swarm Listener](http://swarmlistener.dockerflow.com/) comes into play. It solves some of the problems we discussed. It'll detect new or updated services and propagate all the information to all those that require it. That assumes that the receiver of that information is capable of utilizing that information and reconfiguring itself.

Before we move into a brave new world, let's explore how would we do a traditional configuration of a third-party service. We'll use Prometheus as an example.

## Setting Up Prometheus

Let's start by creating a Prometheus service. We'll start small and move slowly toward a more robust solution.

> If you are a Windows user, please run all the examples from *Git Bash* (installed through *Git*).

```bash
# TODO: Create a cluster
```

We'll start by downloading the Docker Stack file [prometheus.yml](https://github.com/vfarcic/docker-flow-stacks/blob/master/metrics/prometheus.yml) that provides a very basic definition of a Prometheus service.

```bash
curl -o monitor.yml \
    https://raw.githubusercontent.com/vfarcic/docker-flow-stacks/master/metrics/prometheus.yml
```

The stack is as follows.

```
version: "3"

services:

  prometheus:
    image: prom/prometheus
    ports:
      - 9090:9090
```

As you can see, it is as simple as it can get. It specifies the image and the port that should be opened.

Let's deploy the stack.

```bash
docker stack deploy -c monitor.yml monitor
```

Please wait a few moments until the image is pulled and deployed. You can monitor the status by executing the `docker stack ps monitor` command.

> If you're a windows user, Git Bash might not be able to use the `open` command. If that's the case, open the addresses from the commands that follow directly in your browser of choice.

Let's confirm that Prometheus service is indeed up-and-running.

```bash
open "http://localhost:9090"
```

You should see the Prometheus Graph screen.

Let's take a look at the configuration.

```bash
open "http://localhost:9090/config"
```

TODO: Screenshot

You should see the default config that does not define much more than intervals and internal scraping. In its current state, Prometheus is not very useful.

We should start fine tuning it. There are quite a few ways we can do that.

We can create a new Docker image that would extend the one we used and add our own configuration file. That solution has a distinct advantage of being immutable and, hence, very reliable. Since Docker image cannot be changed, we can guarantee that the configuration is exactly as we want it to be no matter where we deploy it. If the service fails, Swarm will reschedule it and, since the configuration is baked into the image, it'll be preserved. The problem with this approach is that it is not suitable for microservices architecture. If Prometheus has to be reconfigured with every new service (or at least those that expose metrics), we would need to build it quite often and tie that build to CD processes executed for the services we're developing. This approach is suitable only to a relativelly static cluster and monolithic applications. Discarded!

We can enter a running Prometheus container, modify its configuration, and reload it. While this allows a higher level of dynamism, it is not fault tolerant. If Prometheus fails, Swarm will reschedule it, and all the changes we made will be lost. Besides fault tolerance, modifying a config in a running container poses additional problems when running it as a service inside a cluster. We need to find out the node it is running in, SSH into it, figure out the ID of the container, and, only than, we can `exec` into it and modify the config. While those steps are not overly complex and can be scripted, they will pose an unnecessary complexity. Discarded!

We could mount a network volume to the service. TODO: Continue

```bash
docker stack rm monitor
```

## Deploying Docker Flow Monitor

```bash
docker network create -d overlay monitor

curl -o monitor.yml \
    https://raw.githubusercontent.com/vfarcic/docker-flow-stacks/master/metrics/docker-flow-monitor.yml
```

```
version: "3"

services:

  prometheus:
    image: vfarcic/docker-flow-monitor:${TAG:-latest}
    environment:
      - GLOBAL_SCRAPE_INTERVAL=10s
    ports:
      - 9090:9090
```

```bash
docker stack deploy -c monitor.yml monitor

docker stack ps monitor

open "http://localhost:9090/config"
```

![Configuration defined through environment variables](img/env-to-config.png)

```bash
docker service rm monitor
```

## Integration With Docker Flow Proxy

```bash
docker network create -d overlay proxy

curl -o proxy.yml \
    https://raw.githubusercontent.com/vfarcic/docker-flow-stacks/master/proxy/docker-flow-proxy.yml

docker stack deploy -c proxy.yml proxy

docker stack ps proxy

curl -o monitor.yml \
    https://raw.githubusercontent.com/vfarcic/docker-flow-stacks/master/metrics/docker-flow-monitor-proxy.yml
```

```
version: "3"

services:

  prometheus:
    image: vfarcic/docker-flow-monitor:${TAG:-latest}
    environment:
      - GLOBAL_SCRAPE_INTERVAL=10s
      - ARG_WEB_ROUTE-PREFIX=/monitor
      - ARG_WEB_EXTERNAL-URL=http://${DOMAIN:-localhost}/monitor
    network:
      - proxy
      - default
    deploy:
      labels:
        - com.df.notify=true
        - com.df.distribute=true
        - com.df.servicePath=/monitor
        - com.df.serviceDomain=${DOMAIN:-localhost}
        - com.df.port=9090

  swarm-listener:
    image: vfarcic/docker-flow-swarm-listener
    networks:
      - proxy
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
    environment:
      - DF_NOTIFY_CREATE_SERVICE_URL=http://monitor:8080/v1/docker-flow-monitor/reconfigure
      - DF_NOTIFY_REMOVE_SERVICE_URL=http://monitor:8080/v1/docker-flow-monitor/remove
    deploy:
      placement:
        constraints: [node.role == manager]

networks:
  default:
    external: false
  proxy:
    external: true
```

* NOTE: Swarm listeners can be combined

```bash
docker stack deploy -c monitor.yml monitor

docker stack ps monitor

open "http://localhost/monitor"
```

![Prometheus integrated with Docker Flow Proxy](img/with-proxy.png)

## Integration With Exporters

```bash
curl -o exporters.yml \
    https://raw.githubusercontent.com/vfarcic/docker-flow-stacks/master/metrics/exporters.yml
```

```
version: "3"

services:

  ha-proxy:
    image: quay.io/prometheus/haproxy-exporter:${HA_PROXY_TAG:-latest}
    networks:
      - proxy
      - monitor
    deploy:
      labels:
        - com.df.notify=true
        - com.df.scrapePort=9101
    command: -haproxy.scrape-uri="http://admin:admin@proxy/admin?stats;csv"

  cadvisor:
    image: google/cadvisor:${CADVISOR_TAG:-latest}
    networks:
      - monitor
    volumes:
      - /:/rootfs
      - /var/run:/var/run
      - /sys:/sys
      - /var/lib/docker:/var/lib/docker
    deploy:
      mode: global
      labels:
        - com.df.notify=true
        - com.df.scrapePort=8080

networks:
  monitor:
    external: true
  proxy:
    external: true
```

```bash
docker stack deploy -c exporters.yml exporter

docker stack ps exporter

open "http://localhost/monitor/config"
```

![Configuration with exporters](img/exporters.png)

```bash
curl -o go-demo.yml \
    https://raw.githubusercontent.com/vfarcic/go-demo/master/docker-compose-stack.yml

docker stack deploy -c go-demo.yml go-demo

docker stack ps go-demo

curl -i "http://localhost/demo/hello"

open "http://localhost/monitor/graph"

# haproxy_backend_connections_total

for ((n=0;n<200;n++)); do
    curl "http://localhost/demo/hello"
done

open "http://localhost/monitor/graph"

# haproxy_backend_connections_total

open "http://localhost/monitor/graph"

# container_memory_usage_bytes{container_label_com_docker_swarm_service_name="go-demo_main"}

for ((n=0;n<200;n++)); do
    curl "http://localhost/demo/hello"
done

open "http://localhost/monitor/graph"

# container_memory_usage_bytes{container_label_com_docker_swarm_service_name="go-demo_main"}
```