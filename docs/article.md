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

We could mount a network volume to the service. That would solve persistence, but would still leave the problem created by a dynamic nature of a cluster. We still, potentially, need to change the configuration and reload Prometheus every time a new service is deployed or updated. Wouldn't it be great if Prometheus could be configured through environment variables instead configuration files? That would make it more "container native". How about an API that would allow us to add a scrape target or an alert? If that sounds like something that would be an interesting addition to Prometheus, this is your lucky day. Just read on.

## Deploying Docker Flow Monitor

Deploying *Docker Flow Monitor* is easy (almost all Docker services are). We'll start by creating a network called `monitor`. We could let Docker stack create it for us but it is useful to have it defined externally so that we can easily attach it to services from other stacks.

```bash
docker network create -d overlay monitor
```

Now we can download the stack.

```bash
curl -o monitor.yml \
    https://raw.githubusercontent.com/vfarcic/docker-flow-stacks/master/metrics/docker-flow-monitor.yml
```

The stack is as follows.

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

The environment variable `GLOBAL_SCRAPE_INTERVAL` shows the first improvement over the "original" Prometheus service. It allows us to define entries of its configuration as environment variables. This, in itself, is not a big improvement. More powerful additions will be presented later on.

TODO: Link to env. vars. docs.

Now we're ready to deploy the stack.

```bash
docker stack deploy -c monitor.yml monitor
```

Please wait a few moments until Swarm pulls the image and starts the service. You can monitor the status by executing the `docker stack ps monitor` command.

Once the service is running, we can confirm that the environment variable indeed generated the configuration.

```bash
open "http://localhost:9090/config"
```

![Configuration defined through environment variables](img/env-to-config.png)

## Integrating Docker Flow Monitor With Docker Flow Proxy

Having a port opened (other than `80` and `443`) is generally not a good idea. If for no other reason, at least because its not user friendly to remember a different port for each service. To mitigate this, we'll integrate *Docker Flow Monitor* with [Docker Flow Proxy](http://proxy.dockerflow.com/).

```bash
docker network create -d overlay proxy

curl -o proxy.yml \
    https://raw.githubusercontent.com/vfarcic/docker-flow-stacks/master/proxy/docker-flow-proxy.yml

docker stack deploy -c proxy.yml proxy
```

We create the `proxy` network, downloaded the [docker-flow-proxy.yml](https://github.com/vfarcic/docker-flow-stacks/blob/master/proxy/docker-flow-proxy.yml) stack, and deployed it.

With the proxy up and runnin, we should redeploy our monitor. This time it will not expose port `9090`.

We'll start by downloading the [docker-flow-monitor-proxy.yml](https://github.com/vfarcic/docker-flow-stacks/blob/master/metrics/docker-flow-monitor-proxy.yml) start.

```bash
curl -o monitor.yml \
    https://raw.githubusercontent.com/vfarcic/docker-flow-stacks/master/metrics/docker-flow-monitor-proxy.yml
```

The stack is as follows.

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

This time we added a few additional environment variables. They will be used instead the Prometheus startup arguments. We are specifying the route prefix (`/monitor`) as well as the full external URL.

We also used the environment variables `com.df.*` that will tell the proxy how to reconfigure itself so that Prometheus is available through the path `/monitor`.

TODO: Link to environment variables documentation.

The second service is [Docker Flow Swarm Listener](http://swarmlistener.dockerflow.com/) that will listen to Swarm events and send reconfigure and remove requests to the monitor. You'll see its usage soon.

Let us deploy the new version of the monitor stack.

```bash
docker stack deploy -c monitor.yml monitor
```

Please execute `docker stack ps monitor` to check the status of the stack. Once it's up-and-running, we can confirm that the monitor is indeed integrated with the proxy.

```bash
open "http://localhost/monitor"
```

![Prometheus integrated with Docker Flow Proxy](img/with-proxy.png)

Now it's time to start exploring the exporters and their integration with *Docker Flow Monitor*.

## Integrating Docker Flow Monitor With Exporters

We'll start by downloading the [exporters.yml](https://github.com/vfarcic/docker-flow-stacks/blob/master/metrics/exporters.yml) stack.

```bash
curl -o exporters.yml \
    https://raw.githubusercontent.com/vfarcic/docker-flow-stacks/master/metrics/exporters.yml
```

The stack is as follows.

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

  node-exporter:
    image: basi/node-exporter:${NODE_EXPORTER_TAG:-v1.13.0}
    networks:
      - monitor
    environment:
      - HOST_HOSTNAME=/etc/host_hostname
    volumes:
      - /proc:/host/proc
      - /sys:/host/sys
      - /:/rootfs
      - /etc/hostname:/etc/host_hostname
    deploy:
      mode: global
      labels:
        - com.df.notify=true
        - com.df.scrapePort=9100
    command: '-collector.procfs /host/proc -collector.sysfs /host/proc -collector.filesystem.ignored-mount-points "^/(sys|proc|dev|host|etc)($$|/)" -collector.textfile.directory /etc/node-exporter/ -collectors.enabled="conntrack,diskstats,entropy,filefd,filesystem,loadavg,mdadm,meminfo,netdev,netstat,stat,textfile,time,vmstat,ipvs"'

networks:
  monitor:
    external: true
  proxy:
    external: true
```

As you can see, it contains `haproxy` and `node` exporters as well as `cadvisor`. The `haproxy-exporter` will provide metrics related to *Docker Flow Proxy* (it uses HAProxy in the background). `cadvisor` will provide the information about the containers inside our cluster. Finally, `node-exporter` collects server metrics. You'll notice that `cadvisor` and `node-exporter` are running in the `global mode`. A replica will run on each server so that we can obtain an accurate picture of the whole cluster.

The important part of the stack definition are `com.df.notify` and `com.df.scrapePort` labels. The first one tells `swarm-listener` that it should notify the monitor when those services are created (or destroyed). The `scrapePort` is the port of the exporters.

Let's deploy the stack and see it in action.

```bash
docker stack deploy -c exporters.yml exporter
```

Please wait until all the services in the stack and running. You can monitor their status with the `docker stack ps exporter` command.

Once the `exporters` stack is up-and-running, we can confirm that they were added to the `monitor` config.

```bash
open "http://localhost/monitor/config"
```

![Configuration with exporters](img/exporters.png)

We can also confirm that all the targets are indeed working.

```bash
open "http://localhost/monitor/targets"
```

TODO: Targets screenshot

If we used the "official" Prometheus image. Setting up those targets would require update to the config file and reload of the service. On top of that, we'd need to persist the configuration. Instead, we let Swarm Listener notify the Monitor that there are new services that should, in this case, generate new scraping targets. Instead splitting the initial information into multiple locations, we specified scraping info as service labels.

Now that targets are configured and scraping data, we should generate some traffic that would let us see the metrics in action.

We'll deploy the [go-demo stack](https://github.com/vfarcic/go-demo/blob/master/docker-compose-stack.yml). It contains a service with an API and a corresponding database.

```bash
curl -o go-demo.yml \
    https://raw.githubusercontent.com/vfarcic/go-demo/master/docker-compose-stack.yml

docker stack deploy -c go-demo.yml go-demo
```

As before, we should wait a few moments for the service to become operational. Please execute `docker stack ps go-demo` to confirm that all the replicas are running.

Now that the demo service is running, we can explore some of the metrics we have at our disposal.

```bash
open "http://localhost/monitor/graph"
```

Please type `haproxy_backend_connections_total` in the *Expression* field and press the *Execute* button. The result should be zero connections. Let's spice it up by creating a bit of traffic.

```bash
for ((n=0;n<200;n++)); do
    curl "http://localhost/demo/hello"
done
```

We sent 200 requests to the `go-demo` service.

If we go back to the Prometheus UI and repeat the execution of the `haproxy_backend_connections_total` expression, the result should be different. The result will be different from one machine to another. In my case, there are 67 backend connections.

![HA Proxy metrics](img/haproxy-backend-connections-total.png)

We could display the data as a graph by clicking the *Graph* tab. I recommend using Grafana instead but that's out of the scope of this text.

How about memory usage? We have the data through `cadvisor` scraping so we might just as well use it.

Please type `container_memory_usage_bytes{container_label_com_docker_swarm_service_name="go-demo_main"}` in the expression field and click
the *Execute* button.

The result is memory usage limited to the Docker service `go-demo_main`. Depending on the view, you should see three values in *Console* or three lines in the *Graph* tab. They represent memory usage of the three replicas of the `go-demo_main` service.

![cAdvisor metrics](img/container-memory-usage-bytes.png)

Finally, let's explore one of the `node-exporter` metrics. We can, for example, display the amount of free memory on each node.

Please type `sum by (instance) (node_memory_MemFree)` in the expression field and click the *Execute* button.

The result is representation of free memory for each of the nodes of the cluster.

TODO: Screenshot

## Integrating Docker Flow Monitor With Alerts

Monitoring systems are not meant to be a substitute for Netflix. They are not meant to be watched. Instead, they should collect data and, if certain conditions are met, create alerts.

Let us create the first alert. We'll update our `go-demo_main` service by adding a few labels.

```bash
docker service update \
    --label-add com.df.alertName=mem \
    --label-add com.df.alertIf='container_memory_usage_bytes{container_label_com_docker_swarm_service_name="go-demo_main"} > 20000000' \
    go-demo_main
```

> Normally, you should have labels defined inside your stack file. However, since we'll do quite a few iterations with different values, we'll updating the service instead modifying the stack file.


The label `com.df.alertName` is the name of the alert. It will be prefixed with the name of the service stripped from underscores and dashes (`godemomem`). That way, unique alert name is guaranteed.

The second label (`com.df.alertIf`) is more important. If defines the expression. Translated to plain words, it take the memory usage limited to the `go-demo_main` service and checked whether it is bigger than 20MB (20000000 bytes). An alert will be launched if the expressions is true.

Let's take a look at the configuration.

```bash
open "http://localhost/monitor/config"
```

As you can see, `alert.rules` file was added to the `rule_files` section.

![Configuration with alert rules](img/config-with-alert-rules.png)

Let us explore the rules we created by now.

```bash
open "http://localhost/monitor/rules"
```

The expression we specified with the `com.df.alertIf` label reached *Docker Flow Monitor*.

![Rules with go-demo memory usage](img/rules-go-demo-memory.png)

Finally, let's take a look at the alerts.

```bash
open "http://localhost/monitor/alerts"
```

The *godemomainmem* alert is green meaning that none of the `go-demo_main` containers are using over 20MB of memory. Please click the *godemomainmem* link to expand the alert definition.

![Alerts with go-demo memory usage](img/alerts-go-demo-memory.png)

The alert is green meaning that the service uses less than 20MB of memory. If we'd like to see how much memory it actually uses we need to go back to the graph screen.

```bash
open "http://localhost/monitor/graph"
```

Once inside the graph screen, please type the expression that follows and press the *Execute* button.

```
container_memory_usage_bytes{container_label_com_docker_swarm_service_name="go-demo_main"}
```

The exact value will vary from one case to another. No matter which one you got, it should be below 20MB.

Let's change the alert so that it is triggered when `go-demo_main` service uses more than 1MB.

```bash
docker service update \
    --label-add com.df.alertName=mem \
    --label-add com.df.alertIf='container_memory_usage_bytes{container_label_com_docker_swarm_service_name="go-demo_main"} > 1000000' \
    go-demo_main
```

Since we are updating the same service and using the same `alertName`, the previous alert definition was overwritten with the new one.

Let's go back to the alerts screen.

```bash
open "http://localhost/monitor/alerts"
```

This time, the alert is read meaning that the condition is fulfilled. Our service is using more than 1MB of memory. Please click the *godemomainmem* link to expand the alert and see more details.

![Alerts with go-demo memory usage in firing state](img/alerts-go-demo-memory-firing.png)

TODO: Continue

```bash
docker service update \
    --limit-memory 20mb \
    --reserve-memory 10mb \
    go-demo

open "http://localhost/monitor/graph"

# container_spec_memory_limit_bytes{container_label_com_docker_swarm_service_name="go-demo"}
# container_memory_usage_bytes{container_label_com_docker_swarm_service_name="go-demo"}

docker service update \
    --label-add com.df.alertName=memlimit \
    --label-add com.df.alertIf='container_memory_usage_bytes{container_label_com_docker_swarm_service_name="go-demo"}/container_spec_memory_limit_bytes{container_label_com_docker_swarm_service_name="go-demo"} > 0.1' \
    go-demo

open "http://localhost/monitor/alerts"

docker service update \
    --label-add com.df.alertName=memlimit \
    --label-add com.df.alertIf='container_memory_usage_bytes{container_label_com_docker_swarm_service_name="go-demo"}/container_spec_memory_limit_bytes{container_label_com_docker_swarm_service_name="go-demo"} > 0.8' \
    go-demo

open "http://localhost/monitor/alerts"

docker service update \
    --label-add com.df.alertName=memlimit \
    --label-add com.df.alertIf='container_memory_usage_bytes{container_label_com_docker_swarm_service_name="go-demo"}/container_spec_memory_limit_bytes{container_label_com_docker_swarm_service_name="go-demo"} > 0.8' \
    --label-add com.df.alertFor='1m' \
    go-demo

open "http://localhost/monitor/alerts"
```

NOTE: Rules can be added to exporters as well
