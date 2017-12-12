## Are We Ready For Microservices?

Microservices, microservices, microservices. We are all in the process of rewriting or planning to rewrite our monoliths into microservices. Some already did it. We are putting them into containers and deploying them through one of the schedulers. We are marching into a glorious future. There's nothing that can stop us now. Except... We, as an industry, are not yet ready for microservices. One thing is to design our services in a way that they are stateless, fault tolerant, scalable, and so on.

Unless you just started a new project, chances are that you still did not reach that point and that there are quite a few legacy services floating around. However, for the sake of brevity and the urge to get to the point I'm trying to make, I will assume that all the services you're in control of are truly microservices. Does that mean that the whole system reached that nirvana state? Is deployment of a service (no matter who wrote it) fully independent from the rest of the system? Most likelly it isn't.

Let's say that you just finished the first release of your new service. Since you are practicing continuous deployment, that first release is actually the first commit to your code repository. Your CD tool of choice detected that change, and started the process. At the end of it, the service is deployed to production. I can see a smile on your face. It's that expression of happiness that can be seen only after a child is born or a service is deployed to production for the first time. That smile should not be long lasting since deploying a service is only the beginning. It needs to be integrated with the rest of the system. The proxy needs to be reconfigured. Logs parser needs to be updated with the format produced by the new service. Monitoring system needs to become aware of the new service. Alerts need to be created with the goal of sending warning and error messages when the state of the service reaches certain thresholds. The whole system needs to adapt to the new service and incorporate the new variables introduced with the commit we made a few moments ago.

How to we adapt the system so that it takes the new service into account? How do we make that service be the integral part of the system?

Unless you are writing everything yourself (in which case you must be Google), your system consists of a mixture of services written by you and services written and maintained by others. You probably use a third party proxy (hopefully that's [Docker Flow Proxy](http://proxy.dockerflow.com/)). You might have chosen the ELK stack for centralized logging. How about monitoring? It could be Prometheus. No matter the choices you made, you are not in control of the architecture of the whole system. Heck, you're probably not even in control of all the services your wrote.

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

We'll start by cloning [vfarcic/docker-flow-monitor](https://github.com/vfarcic/docker-flow-monitor) repository. It contains all the scripts and Docker stacks we'll use throughout this article.

```bash
git clone https://github.com/vfarcic/docker-flow-monitor.git

cd docker-flow-monitor
```

Before we create a Prometheus service, we need to have a cluster. We'll create three nodes using Docker Machine. Feel free to skip the commands that follow if you already have a working Swarm cluster.

> If you are a Windows user, please run all the examples from *Git Bash* (installed through *Git*) or any other Bash you might have.

```bash
chmod +x scripts/dm-swarm.sh

./scripts/dm-swarm.sh

eval $(docker-machine env swarm-1)
```

The `dm-swarm.sh` scripts created the nodes and joined them into a Swarm cluster.

Now we can create the first Prometheus service. We'll start small and move slowly toward a more robust solution.

We'll start with the stack defined in `stacks/prometheus.yml`. It is as follows.

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
docker stack deploy -c stacks/prometheus.yml monitor
```

Please wait a few moments until the image is pulled and deployed. You can monitor the status by executing the `docker stack ps monitor` command.

Let's confirm that Prometheus service is indeed up-and-running.

> If you're a windows user, Git Bash might not be able to use the `open` command. If that's the case, when you see the `open`, open the addresses from those commands directly in your browser of choice.

```bash
open "http://$(docker-machine ip swarm-1):9090"
```

You should see the Prometheus Graph screen.

Let's take a look at the configuration.

```bash
open "http://$(docker-machine ip swarm-1):9090/config"
```

You should see the default config that does not define much more than intervals and internal scraping. In its current state, Prometheus is not very useful.

![Prometheus with default configuration](img/prometheus-default-config.png)

We should start fine tuning it. There are quite a few ways we can do that.

We can create a new Docker image that would extend the one we used and add our own configuration file. That solution has a distinct advantage of being immutable and, hence, very reliable. Since Docker image cannot be changed, we can guarantee that the configuration is exactly as we want it to be no matter where we deploy it. If the service fails, Swarm will reschedule it and, since the configuration is baked into the image, it'll be preserved. The problem with this approach is that it is not suitable for microservices architecture. If Prometheus has to be reconfigured with every new service (or at least those that expose metrics), we would need to build it quite often and tie that build to CD processes executed for the services we're developing. This approach is suitable only to a relativelly static cluster and monolithic applications. Discarded!

We can enter a running Prometheus container, modify its configuration, and reload it. While this allows a higher level of dynamism, it is not fault tolerant. If Prometheus fails, Swarm will reschedule it, and all the changes we made will be lost. Besides fault tolerance, modifying a config in a running container poses additional problems when running it as a service inside a cluster. We need to find out the node it is running in, SSH into it, figure out the ID of the container, and, only than, we can `exec` into it and modify the config. While those steps are not overly complex and can be scripted, they will pose an unnecessary complexity. Discarded!

We could mount a network volume to the service. That would solve persistence, but would still leave the problem created by a dynamic nature of a cluster. We still, potentially, need to change the configuration and reload Prometheus every time a new service is deployed or updated. Wouldn't it be great if Prometheus could be configured through environment variables instead configuration files? That would make it more "container native". How about an API that would allow us to add a scrape target or an alert? If that sounds like something that would be an interesting addition to Prometheus, this is your lucky day. Just read on.

## Deploying Docker Flow Monitor

Deploying *Docker Flow Monitor* is easy (almost all Docker services are). We'll start by creating a network called `monitor`. We could let Docker stack create it for us but it is useful to have it defined externally so that we can easily attach it to services from other stacks.

```bash
docker network create -d overlay monitor
```

The stack is as follows.

```
version: "3"

services:

  monitor:
    image: vfarcic/docker-flow-monitor:${TAG:-latest}
    environment:
      - GLOBAL_SCRAPE_INTERVAL=10s
    ports:
      - 9090:9090
```

The environment variable `GLOBAL_SCRAPE_INTERVAL` shows the first improvement over the "original" Prometheus service. It allows us to define entries of its configuration as environment variables. This, in itself, is not a big improvement. More powerful additions will be presented later on. Please visit [TODO](TODO) for more information about environment variables that can be used as configuration.

Now we're ready to deploy the stack.

```bash
docker stack rm monitor

docker stack deploy \
    -c stacks/docker-flow-monitor.yml \
    monitor
```

Please wait a few moments until Swarm pulls the image and starts the service. You can monitor the status by executing the `docker stack ps monitor` command.

Once the service is running, we can confirm that the environment variable indeed generated the configuration.

```bash
open "http://$(docker-machine ip swarm-1):9090/config"
```

![Configuration defined through environment variables](img/env-to-config.png)

## Integrating Docker Flow Monitor With Docker Flow Proxy

Having a port opened (other than `80` and `443`) is generally not a good idea. If for no other reason, at least because its not user friendly to remember a different port for each service. To mitigate this, we'll integrate *Docker Flow Monitor* with [Docker Flow Proxy](http://proxy.dockerflow.com/).

```bash
docker network create -d overlay proxy

docker stack deploy -c stacks/docker-flow-proxy.yml proxy
```

We created the `proxy` network and deployed the `docker-flow-proxy.yml` stack. We won't go into details how *Docker Flow Proxy* works. The essence is that it will configure itself with each service that has specific labels. Please visit [Docker Flow Proxy documentation](http://proxy.dockerflow.com/) for more information and examples.

With the proxy up and running, we should redeploy our monitor. This time it will not expose port `9090`.

We'll replace the current monitor stack with a new one. The major difference is that, this time, we'll define startup arguments as well as the labels that will allow the proxy to reconfigure itself.

The stack is as follows.

```
version: "3"

services:

  monitor:
    image: vfarcic/docker-flow-monitor:${TAG:-latest}
    environment:
      - GLOBAL_SCRAPE_INTERVAL=10s
      - ARG_WEB_ROUTE-PREFIX=/monitor
      - ARG_WEB_EXTERNAL-URL=http://${DOMAIN:-localhost}/monitor
    networks:
      - proxy
      - monitor
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
      - monitor
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
    environment:
      - DF_NOTIFY_CREATE_SERVICE_URL=http://monitor:8080/v1/docker-flow-monitor/reconfigure
      - DF_NOTIFY_REMOVE_SERVICE_URL=http://monitor:8080/v1/docker-flow-monitor/remove
    deploy:
      placement:
        constraints: [node.role == manager]

networks:
  monitor:
    external: true
  proxy:
    external: true
```

This time we added a few additional environment variables. They will be used instead the Prometheus startup arguments. We are specifying the route prefix (`ARG_WEB_ROUTE-PREFIX`) as well as the full external URL (`ARG_WEB_EXTERNAL-URL`). Please visit [TODO](TODO) for more information about environment variables that can be used as startup arguments.

We also used the environment variables `com.df.*` that will tell the proxy how to reconfigure itself so that Prometheus is available through the path `/monitor`.

The second service is [Docker Flow Swarm Listener](http://swarmlistener.dockerflow.com/) that will listen to Swarm events and send reconfigure and remove requests to the monitor. You'll see its usage soon.

Let us deploy the new version of the monitor stack.

```bash
docker stack rm monitor

DOMAIN=$(docker-machine ip swarm-1) \
    docker stack deploy \
    -c stacks/docker-flow-monitor-proxy.yml \
    monitor
```

In the "real-world" situation, you should use your domain (e.g. `monitor.acme.com`) and would not need `ARG_WEB_ROUTE-PREFIX` and `com.df.servicePath` set to `/monitor`. However, since we do not have a domain for this exercise, we used the IP of `swarm-1` instead.

Please execute `docker stack ps monitor` to check the status of the stack. Once it's up-and-running, we can confirm that the monitor is indeed integrated with the proxy.

```bash
open "http://$(docker-machine ip swarm-1)/monitor/flags"
```

By opening the *flags* screen, not only that we confirmed that the integration with *Docker Flow Proxy* worked but also that the arguments we specified as environment variables are properly propagated. You can observe that through the values of the `web.external-url` and `web.route-prefix` flags.

![Prometheus flags screen with values passed through environment variables](img/flags-web.png)

Now it's time to start exploring the exporters and their integration with *Docker Flow Monitor*.

## Integrating Docker Flow Monitor With Exporters

Now we can deploy a few exporters. They will provide data Prometheus can scrape and put into its database.

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

As you can see, the stack definition contains `haproxy` and `node` exporters as well as `cadvisor`. The `haproxy-exporter` will provide metrics related to *Docker Flow Proxy* (it uses HAProxy in the background). `cadvisor` will provide the information about the containers inside our cluster. Finally, `node-exporter` collects server metrics. You'll notice that `cadvisor` and `node-exporter` are running in the `global mode`. A replica will run on each server so that we can obtain an accurate picture of the whole cluster.

The important part of the stack definition are `com.df.notify` and `com.df.scrapePort` labels. The first one tells `swarm-listener` that it should notify the monitor when those services are created (or destroyed). The `scrapePort` is the port of the exporters.

Let's deploy the stack and see it in action.

```bash
docker stack deploy \
    -c stacks/exporters.yml \
    exporter
```

Please wait until all the services in the stack and running. You can monitor their status with the `docker stack ps exporter` command.

Once the `exporters` stack is up-and-running, we can confirm that they were added to the `monitor` config.

```bash
open "http://$(docker-machine ip swarm-1)/monitor/config"
```

![Configuration with exporters](img/exporters.png)

We can also confirm that all the targets are indeed working.

```bash
open "http://$(docker-machine ip swarm-1)/monitor/targets"
```

As you can see, we have three targets. Two of them (`exporter_cadvisor` and `exporter_node-exporter`) are running as global services. As a result, each has three endpoints, one on each node. The last target is `exporter_ha-proxy`. Since we did not run it globally nor specified multiple replicas, in has only one endpoint.

![Targets and endpoints](img/targets.png)

If we used the "official" Prometheus image. Setting up those targets would require update to the config file and reload of the service. On top of that, we'd need to persist the configuration. Instead, we let Swarm Listener notify the Monitor that there are new services that should, in this case, generate new scraping targets. Instead splitting the initial information into multiple locations, we specified scraping info as service labels.

Now that targets are configured and scraping data, we should generate some traffic that would let us see the metrics in action.

We'll deploy the go-demo stack. It contains a service with an API and a corresponding database. We'll use it as a demo service that will allow us to explore some of the metrics we can use.

```bash
docker stack deploy -c stacks/go-demo.yml go-demo
```

As before, we should wait a few moments for the service to become operational. Please execute `docker stack ps go-demo` to confirm that all the replicas are running.

Now that the demo service is running, we can explore some of the metrics we have at our disposal.

```bash
open "http://$(docker-machine ip swarm-1)/monitor/graph"
```

Please wait a few moments for metrics to start coming in and type `haproxy_backend_connections_total` in the *Expression* field and press the *Execute* button. The result should be zero connections on the backend `go-demo_main-be8080`. Let's spice it up by creating a bit of traffic.

```bash
for ((n=0;n<200;n++)); do
    curl "http://$(docker-machine ip swarm-1)/demo/hello"
done
```

We sent 200 requests to the `go-demo` service.

If we go back to the Prometheus UI and repeat the execution of the `haproxy_backend_connections_total` expression, the result should be different. The result will be different from one machine to another. In my case, there are 67 backend connections.

![HA Proxy metrics](img/haproxy-backend-connections-total.png)

We could display the data as a graph by clicking the *Graph* tab. I recommend using Grafana instead but that's out of the scope of this text.

How about memory usage? We have the data through `cadvisor` scraping so we might just as well use it.

Please type `container_memory_usage_bytes{container_label_com_docker_swarm_service_name="go-demo_main"}` in the expression field and click the *Execute* button.

The result is memory usage limited to the Docker service `go-demo_main`. Depending on the view, you should see three values in *Console* or three lines in the *Graph* tab. They represent memory usage of the three replicas of the `go-demo_main` service.

![cAdvisor metrics](img/container-memory-usage-bytes.png)

Finally, let's explore one of the `node-exporter` metrics. We can, for example, display the amount of free memory on each node.

Please type `sum by (instance) (node_memory_MemFree)` in the expression field and click the *Execute* button.

The result is representation of free memory for each of the nodes of the cluster.

![Graph with available memory](img/graph-memory.png)

## Integrating Docker Flow Monitor With Alerts

Monitoring systems are not meant to be a substitute for Netflix. They are not meant to be watched. Instead, they should collect data and, if certain conditions are met, create alerts.

Let us create the first alert. We'll update our `go-demo_main` service by adding a few labels.

```bash
docker service update \
    --label-add com.df.alertName=mem \
    --label-add com.df.alertIf='container_memory_usage_bytes{container_label_com_docker_swarm_service_name="go-demo_main"} > 20000000' \
    go-demo_main
```

> Normally, you should have labels defined inside your stack file. However, since we'll do quite a few iterations with different values, we'll be updating the service instead modifying the stack file.


The label `com.df.alertName` is the name of the alert. It will be prefixed with the name of the service stripped from underscores and dashes (`godemomem`). That way, unique alert name is guaranteed.

The second label (`com.df.alertIf`) is more important. If defines the expression. Translated to plain words, it take the memory usage limited to the `go-demo_main` service and checked whether it is bigger than 20MB (20000000 bytes). An alert will be launched if the expressions is true.

Let's take a look at the configuration.

```bash
open "http://$(docker-machine ip swarm-1)/monitor/config"
```

As you can see, `alert.rules` file was added to the `rule_files` section.

![Configuration with alert rules](img/config-with-alert-rules.png)

Let us explore the rules we created by now.

```bash
open "http://$(docker-machine ip swarm-1)/monitor/rules"
```

The expression we specified with the `com.df.alertIf` label reached *Docker Flow Monitor*.

![Rules with go-demo memory usage](img/rules-go-demo-memory.png)

Finally, let's take a look at the alerts.

```bash
open "http://$(docker-machine ip swarm-1)/monitor/alerts"
```

The *godemomainmem* alert is green meaning that none of the `go-demo_main` containers are using over 20MB of memory. Please click the *godemomainmem* link to expand the alert definition.

![Alerts with go-demo memory usage](img/alerts-go-demo-memory.png)

The alert is green meaning that the service uses less than 20MB of memory. If we'd like to see how much memory it actually uses we need to go back to the graph screen.

```bash
open "http://$(docker-machine ip swarm-1)/monitor/graph"
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
open "http://$(docker-machine ip swarm-1)/monitor/alerts"
```

This time, the alert is read meaning that the condition is fulfilled. If it is still green, please wait for a few moments and refresh your screen.

Our service is using more than 1MB of memory and, therefore, the alert if statement is fulfilled, and the alert is red.

Please click the *godemomainmem* link to expand the alert and see more details.

![Alerts with go-demo memory usage in firing state](img/alerts-go-demo-memory-firing.png)

While hard-coding limits inside the `alertIf` label works well, we should have limits defined on service level as well. We could, for example, update our service with the `--limit-memory` and `--reserve-memory` flags.

```bash
docker service update \
    --limit-memory 20mb \
    --reserve-memory 10mb \
    go-demo_main
```

From now on, Docker Swarm will have more information how to schedule the service by trying to place each replica to a node that has 10MB available. Moreover, memory of each of those replicas is now limited to 20MB. In other words, we expect the service to use 10MB but would accept up to 20MB.

Let's take a look at the graph screen.

```bash
open "http://$(docker-machine ip swarm-1)/monitor/graph"
```

Please type the expression that follows and press the *Execute* button.

```
container_spec_memory_limit_bytes{container_label_com_docker_swarm_service_name="go-demo_main"}
```

As you can see, memory metric for containers before the last update is set to 0MB, while the limit for the newly created replicas is set to 20MB. Soon, we'll use those metrics to our benefit.

![go-demo memory limit](img/graph-memory-limit.png)

Next we'll check the metrics of the "real" memory usage of the service.

Please type the expression that follows and press the *Execute* button.

```
container_memory_usage_bytes{container_label_com_docker_swarm_service_name="go-demo_main"}
```

Memory consumption will vary from one case to another. In my case it ranges from 1MB to 3.5MB.

![go-demo memory usage](img/graph-memory-usage.png)

If we go back to the `alertIf` label we specified, there is a clear duplication of data. It has the memory limit set to the same value as the `--limit-memory` argument. Duplication is not a good idea because it increases the chances of an error and complicates future updates that need to be performed in multiple places.

A better definition of the `alertIf` statement is as follows.

```bash
docker service update \
    --label-add com.df.alertName=memlimit \
    --label-add com.df.alertIf='container_memory_usage_bytes{container_label_com_docker_swarm_service_name="go-demo"}/container_spec_memory_limit_bytes{container_label_com_docker_swarm_service_name="go-demo"} > 0.8' \
    go-demo_main
```

This time we defined that the `memlimit` alert should be triggered if memory usage is higher than 80% of the memory limit. That way, if, at some later stage, we change the value of the `--limit-memory` argument, the alert will continue working properly.

Let's confirm that *Docker Flow Swarm Listener* sent the notification and that *Docker Flow Monitor* was reconfigured accordingly.

```bash
open "http://$(docker-machine ip swarm-1)/monitor/alerts"
```

Please click the *godemomainmemlimit* link to see the new definition of the alert.

![go-demo alert based on memory limit and usage](img/alert-memory-limit-usage.png)

## Defining Multiple Alerts For A Service

In many cases one alert per service is not enough. We need to be able to define multiple alerts. *Docker Flow Monitor* allows us that by adding index to labels. We can, for example, define labels `com.df.alertName.1`, `com.df.alertName.2`, and `com.df.alertName.3`. As a result, *Docker Flow Monitor* would create three alerts.

Let's see it in action.

We'll update the `node-exporter` service in the `exporter` stack so that it registers two alerts.

```bash
docker service update \
    --label-add com.df.alertName.1=memload \
    --label-add com.df.alertIf.1='(sum by (instance) (node_memory_MemTotal) - sum by (instance) (node_memory_MemFree + node_memory_Buffers + node_memory_Cached)) / sum by (instance) (node_memory_MemTotal) > 0.8' \
    --label-add com.df.alertName.2=diskload \
    --label-add com.df.alertIf.2='(node_filesystem_size{fstype="aufs"} - node_filesystem_free{fstype="aufs"}) / node_filesystem_size{fstype="aufs"} > 0.8' \
    exporter_node-exporter
```

This time, `alertName` and `alertIf` labels got an index suffix (e.g. `.1` and `.2`). The first one (`memload`) will create an alert if memory usage is over 80% of the total available memory. The second alert will create an alert if disk usage is over 80%.

Let's explore the *alerts* screen.

```bash
open "http://$(docker-machine ip swarm-1)/monitor/alerts"
```

As you can see, two new alerts were registered.

![Node exporter alerts](img/alerts-node-exporter.png)

## Failover

What happens if our monitoring solution fails. After all, everything fails sooner or later. Since we're running it as a Swarm service, Docker will reschedule failed services or, to be more precise, failed replicas of a service. But, what happens with our changes to the configuration?

Let's simulate a failure and see the result.

```bash
docker stack rm monitor

DOMAIN=$(docker-machine ip swarm-1) \
    docker stack deploy \
    -c stacks/docker-flow-monitor-proxy.yml \
    monitor
```

We removed the whole stack and deployed it again. This is probably not the best way to simulate a failure but it is probably the simplest one. Otherwise, we'd need to find out on which node the container was running, SSH into that node, and issue `docker container rm [CONTAINER_ID]`. We'll stick with simplicity.

Let's open the home screen.

```bash
open "http://$(docker-machine ip swarm-1)/monitor"
```

It works. Swarm deployed the service for us. There's no surprise there. That's what Swarm does.

Let's see the config screen.

```bash
open "http://$(docker-machine ip swarm-1)/monitor/config"
```

The configuration is there. Most of it is also not a surprise since we are specifying the stack's configuration and startup arguments through environment variables. The part that is not defined in the stack are alert rules. And, yet, they are part of the config.

Let's check the rules screen.

```bash
open "http://$(docker-machine ip swarm-1)/monitor/rules"
```

Rules are there as well. How about alerts?

```bash
open "http://$(docker-machine ip swarm-1)/monitor/alerts"
```

Alerts are defined as well.

How did that happen? How did we recuperate the information for all the services in the cluster? The answer lies in the environment variable `LISTENER_ADDRESS=swarm-listener` defined in the `monitor` stack.

When *Docker Flow Monitor* was initialized, it contacted `swarm-listener` which, through service labels, returned all the information. *Docker Flow Monitor*, in turn, used that information to recreate everything Prometheus needs and restored its previous state.

> **Word of caution**
>
> The fact that *Docker Flow Monitor* restored the Prometheus configuration state does not mean that it restored its data. You might, or might not need the data. Prometheus will start scraping exporters as soon as it is initialized and you might not need historical values. If that's the case, you're all set. If, on the other hand, you do need historical data, please mount the `/prometheus` directory on a network drive.

## Removing Alerts

When a service is removed, all the configuration entries and alerts created through its labels will be removed as well.

Let's try it out.

```bash
docker service rm exporter_node-exporter
```

We removed the `node-exporter` service defined in the `exporter` stack. Now, let's open the *config* screen.

```bash
open "http://$(docker-machine ip swarm-1)/monitor/config"
```

As you can see, the configuration related to the `node-exporter` is gone with the service.

![Configuration without node-exporter](img/config-without-node-exporter.png)

Similarly, the alerts related with the `node-exporters` are gone as well. We can confirm that by opening the *rules* screen.

```bash
open "http://$(docker-machine ip swarm-1)/monitor/rules"
```

The key elements that allow addition and removal of the configuration entries and alert rules are the environment variable `DF_NOTIFY_*_SERVICE_URL` defined in the `stacks/exporters.yml` file.

```
...
    environment:
      - DF_NOTIFY_CREATE_SERVICE_URL=http://monitor:8080/v1/docker-flow-monitor/reconfigure
      - DF_NOTIFY_REMOVE_SERVICE_URL=http://monitor:8080/v1/docker-flow-monitor/remove
...
```

If the environment variable `DF_NOTIFY_CREATE_SERVICE_URL` is specified, *Docker Flow Swarm Listener* will send a notification to the specified addresses (multiple values can be separated with comma). In this case, it sends a service created notification to `http://monitor:8080/v1/docker-flow-monitor/reconfigure`. Similarly, if `DF_NOTIFY_REMOVE_SERVICE_URL` is set, it'll send service removed notifications. On the other hand, *Docker Flow Monitor* (`monitor` service) has a an API that listens to those notifications and updates Prometheus accordingly.

Before we move on, let's recreate the `node-exporter` service we removed. We're still not done with it.

```bash
docker stack deploy \
    -c stacks/exporters-with-labels.yml \
    exporter
```

We can confirm that the `monitor` is reconfigured by opening the *config* and *rules* screens and confirming that `node-exporter` entries are there.

```bash
open "http://$(docker-machine ip swarm-1)/monitor/config"

open "http://$(docker-machine ip swarm-1)/monitor/rules"
```

## Alert Manager

While alerts by themselves are great, they are not very useful unless you're planning to spend all your time in front of the *alerts* screen. There are much better things to stare at. For example, you can watch Netflix instead. It is much more entertaining than Prometheus alerts screen. However, before you start watching Netflix during your working hours, we need to find a wy that you do get notified when an alert is fired.

Where should we send alert messages? Slack is probably a good candidate to start with.

I already created an image based on [prom/alertmanager](https://hub.docker.com/r/prom/alertmanager/). Dockerfile is as follows.

```bash
FROM prom/alertmanager

COPY config.yml /etc/alertmanager/config.yml
```

There's not much mystery there. It extends the official Prometheus AlertManager image and adds a custom config.yml file. Configuration is as follows.

```
route:
  receiver: "slack"
  repeat_interval: 1h

receivers:
    - name: "slack"
      slack_configs:
          - send_resolved: true
            text: "Something horrible happened! Run for your lives!"
            api_url: "https://hooks.slack.com/services/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"
```

The configuration defines `route` with `slack` as the receiver of the alerts. In the `receivers` section we specified that we want resolved notifications (besides alerts), creative text, and the Slack API URL. As a result, alerts will be posted to the *df-monitor-tests* channel in DevOps20 team slack. Please sign up through the [DevOps20 Registration page](http://slack.devops20toolkit.com/) and make sure you join the *df-monitor-tests* channel. This configuration should be more than enough for demo purposes. Later on you should create your own Docker image customized for your own needs.

Please consult [alerting documentation](https://prometheus.io/docs/alerting/configuration/) for more information about Alert Manager configuration options.

The image we'll use is already built and available from [vfarcic/alert-manager:demo](https://hub.docker.com/r/vfarcic/alert-manager/tags).

Next, we'll take a quick look at the *alert-manager-demo.yml* stack.

```
version: "3"

services:

  alert-manager:
    image: vfarcic/alert-manager:demo
    ports:
      - 9093:9093
    networks:
      - monitor

networks:
  monitor:
    external: true
```

The stack is very straighforward. The only thing worth noting is that we are exposing the port `9093` only for demo purposes. Later on, when we integrate it with *Docker Flow Monitor*, they will communicate through the `monitor` network without the need to expose any ports. We need the port `9093` to demonstrate manual triggering of alerts through Alert Manager. We'll get rid of it later on.

Let's deploy the stack.

```bash
docker stack deploy \
    -c stacks/alert-manager-demo.yml \
    alert-manager
```

Please wait a few moments until the service is deployed. You can monitor the status through the `docker stack ps alert-manager` command.

Now we can send a manual request to the *Alert Manager*.

```bash
curl -H "Content-Type: application/json" \
    -d '[{"labels":{"alertname":"My Fancy Alert"}}]' \
    $(docker-machine ip swarm-1):9093/api/v1/alerts
```

Before you execute the request, please change the *My Fancy Alert* name to something else. That way you'll be able to recognize your alert from those submitted by other readers of this tutorial.

Please open *df-monitor-tests* channel in *DevOps20* Slack team and observe that a new notification was posted.

Now that we confirmed that `alert-manager` works when triggered manually, we'll remove the stack and deploy the version integrated with *Docker Flow Monitor*.

```bash
docker stack rm alert-manager
```

We'll deploy the `docker-flow-monitor-full.yml` stack. It container `monitor` and `swarm-listener` services we're already familiar with and adds `alert-manager`. The only change to the `monitor` service is the addition of the environment variable `ALERTMANAGER_URL=http://alert-manager:9093`. It defines the address and the port of the `alert-manager`.

The definition of the `alert-manager` service is as follows.

```
  alert-manager:
    image: vfarcic/alert-manager:demo
    networks:
      - monitor
```

As you can see, it is the same as the one we deployed earlier except that the ports are removed.

Let's deploy the new stack.

```bash
DOMAIN=$(docker-machine ip swarm-1) \
    docker stack deploy \
    -c stacks/docker-flow-monitor-full.yml \
    monitor
```

We should confirm that the `alert-manager` is correctly configured through the environment variable `ALERTMANAGER_URL`.

```bash
open "http://$(docker-machine ip swarm-1)/monitor/flags"
```

As you can see from the *flags* screen, the *alertmanager.url* is now part of the Prometheus configuration.

![Prometheus flags screen with values passed through environment variables](img/flags-alert-manager.png)

Let us generate an alert.

```bash
docker service update \
    --label-add com.df.alertName=memlimit \
    --label-add com.df.alertIf='container_memory_usage_bytes{container_label_com_docker_swarm_service_name="go-demo_main"}/container_spec_memory_limit_bytes{container_label_com_docker_swarm_service_name="go-demo_main"} > 0.1' \
    go-demo_main
```

We updated the `main` service from the `go-demo` stack by adding the `alertIf` label. It defines `memlimit` alert that will be triggered if the service exceeds 10% of the memory limit. In other words, it will almost certainly fire the alert.

Let's open the alerts screen.

```bash
open "http://$(docker-machine ip swarm-1)/monitor/alerts"
```

As you can see, the alert is red (if it isn't, wait a few moments and refresh your screen). Since we configured the *Alert Manager*, the alert was already sent to it and, from there, forwarded to Slack. Please open the *df-monitor-tests* channel in *DevOps20* Slack team and observe that a new notification was posted.

We'll restore the `go-demo` alert to it's original state (used memory over 80%).

```bash
docker service update \
    --label-add com.df.alertName=memlimit \
    --label-add com.df.alertIf='container_memory_usage_bytes{container_label_com_docker_swarm_service_name="go-demo_main"}/container_spec_memory_limit_bytes{container_label_com_docker_swarm_service_name="go-demo_main"} > 0.8' \
    go-demo_main
```

A few moments later, we can observe that the alert is gree again.

```bash
open "http://$(docker-machine ip swarm-1)/monitor/alerts"
```

Since we specified `send_resolved: true` in the `alert-manager` config, we go another notification. This time the message states that the issue is resolved.

The only thing left is to create your own Alert Manager image. If you choose to send alert to Slack (there are many other destinations you can choose from) you'll need Webhook URL. The instructions for obtaining it are as follows.

Please login to your Team Slack channel, open the settings menu by clicking the team name, and select *Apps & integrations*.

![Team setting Slack menu](img/slack-team-setting.png)

You will be presented with the *App Directory* screen. Click the *Manage* link located in the top-right corner of the screen followed with the *Custom Integrations* item in the left-hand menu. Select *Incomming WebHooks* and click the *Add Configuration* button. Select the channel where alerts will be posted and click the *Add Incomming WebHooks integration* button. Copy the *Webhook URL*. We'll need it soon.

We are, finally, done with a very basic introduction to *Docker Flow Monitor* and friends. What you saw here is only the tip of the iceberg. Stay tuned. Much more is coming.

## Cleanup

Before you leave, please remove the Docker machines we created and free the resources.

```bash
docker-machine rm -f swarm-1 swarm-2 swarm-3
```
