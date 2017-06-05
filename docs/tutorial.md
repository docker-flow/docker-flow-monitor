# Running Docker Flow Monitor

The examples that follow assume that you have Docker Machine version v0.8+ that includes Docker Engine v1.12+.

!!! info
	If you are a Windows user, please run all the examples from *Git Bash* (installed through *Docker for Windows*). Also, make sure that your Git client is configured to check out the code *AS-IS*. Otherwise, Windows might change carriage returns to the Windows format.

## Setting Up A Cluster

!!! info
    Feel free to skip this section if you already have a Swarm cluster that can be used for this tutorial

We'll create a Swarm cluster consisting of three nodes created with Docker Machine.

```bash
git clone https://github.com/vfarcic/docker-flow-monitor.git

cd docker-flow-monitor

./scripts/dm-swarm.sh

eval $(docker-machine env swarm-1)
```

We cloned the [vfarcic/docker-flow-monitor](https://github.com/vfarcic/docker-flow-monitor) repository. It contains all the scripts and stack files we'll use throughout this tutorial. Next, we executed the `dm-swarm.sh` script that created the cluster. Finally, we used the `eval` command to tell our local Docker client to use the remote Docker engine `swarm-1`.

Now that the cluster is up-and-running, we can deploy the *Docker Flow Monitor* stack.

## Deploying Docker Flow Monitor

We'll deploy [stacks/docker-flow-monitor-slack.yml](https://github.com/vfarcic/docker-flow-monitor/blob/master/stacks/docker-flow-monitor-slack.yml) stack that contains an example combination of parameters. The stack is as follows.

The stack contains three services; `monitor`, `alert-manager`, and `swarm-listener`. We'll go through each separately.

The definition of the `monitor` service is as follows.

```
  monitor:
    image: vfarcic/docker-flow-monitor
    environment:
      - LISTENER_ADDRESS=swarm-listener
      - GLOBAL_SCRAPE_INTERVAL=10s
      - ARG_ALERTMANAGER_URL=http://alert-manager:9093
    networks:
      - monitor
    ports:
      - 9090:9090
```

The environment variables show the first advantage of using *Docker Flow Monitor* instead directly Prometheus. All the configuration options and startup arguments can be specified as environment variables thus removing the need for configuration files and their persistence.

!!! info
    Please visit [Configuring Docker Flow Monitor](http://monitor.dockerflow.com/config/) for more information about the available options.

The next in line is `alert-manager` service. The definition is as follows.

```
  alert-manager:
    image: vfarcic/alert-manager:slack
    networks:
      - monitor
```

We're using `vfarcic/alert-manager:slack` because it is already preconfigured to send notifications to [DevOps20 Slack](http://slack.devops20toolkit.com/) (channel #df-monitor-tests). Feel free to replace `vfarcic/alert-manager:slack` with your own image based on [prom/alertmanager/](https://hub.docker.com/r/prom/alertmanager/).

Finally, the last service in the stack is `swarm-listener`. The definition is as follows.

```
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
```

The `swarm-listener` service will listen to Swarm events and notify `monitor` whenever a service is created, updated, or removed.

!!! info
    Please visit [Docker Flow Swarm Listener documentation](http://swarmlistener.dockerflow.com/) for more information about the project.

Let's deploy the `monitor` stack

```bash
docker network create -d overlay monitor

docker stack deploy \
    -c stacks/docker-flow-monitor-tutorial.yml \
    monitor
```

Please wait until all the services are running. You can check their statuses by executing `docker stack ps monitor` command.

Now we can open *Prometheus* from a browser.

> If you're a Windows user, Git Bash might not be able to use the `open` command. If that's the case, replace the `open` command with `echo`. As a result, you'll get the full address that should be opened directly in your browser of choice.

```bash
open "http://$(docker-machine ip swarm-1):9090"
```

If you navigate to the *Status > Command-Line Flags* screen, you'll notice that *alertmanager.url* entry is already configured through the environment variable `ARG_ALERTMANAGER_URL`. Similarly, the *Status > Configuration* screen also has the configuration created through the environment variable `GLOBAL_SCRAPE_INTERVAL`.

Now we can start collecting metrics.

## Collecting Metrics And Defining Alerts

Prometheus is a pull system. It scrapes exporters and stores metrics in its internal database.

Let us deploy a few exporters.

We'll deploy exporter stack defined in the [stacks/exporters-tutorial.yml](https://github.com/vfarcic/docker-flow-monitor/blob/master/stacks/exporters-tutorial.yml). It contains two services; `cadvisor` and `node-exporter`.

The definition of the `cadvisor` service is as follows.

```
  cadvisor:
    image: google/cadvisor
    ...
    deploy:
      mode: global
      labels:
        - com.df.notify=true
        - com.df.scrapePort=8080
```

Service labels are what make this service special. `com.df.notify` tells `swarm-listener` that it should notify `monitor` when this service is created, updated, or removed. The `com.df.scrapePort` label specifies that Prometheus should scrape data from this service running on port `8080`.

!!! info
    Please visit [Usage documentation](http://monitor.dockerflow.com/usage/) for more information about the available options.

The second service (`node-exporter`) defines more than scraping port. The definition is as follows.

```
  node-exporter:
    image: basi/node-exporter
    ...
    deploy:
      mode: global
      labels:
        - com.df.notify=true
        - com.df.scrapePort=9100
        - com.df.alertName.1=memload
        - com.df.alertIf.1=(sum by (instance) (node_memory_MemTotal) - sum by (instance) (node_memory_MemFree + node_memory_Buffers + node_memory_Cached)) / sum by (instance) (node_memory_MemTotal) > 0.8
        - com.df.alertName.2=diskload
        - com.df.alertIf.2=@node_fs_limit:0.8
    ...
```

This time, we added a few additional labels. `com.df.alertName.1` will tell Prometheus that it should create an alert called `memload`. The name of the alert is accompanied with the condition specified as `com.df.alertIf.1`. Multiple alerts can be defined by adding labels with incremental indexes. As the second alert, we used `com.df.alertName.2=diskload` to defined the name and `com.df.alertIf.2=@node_fs_limit:0.8` to define the condition. This time, we use one of the shortcuts instead writing the full syntax.

Let's deploy the `exporter` stack.

```bash
docker stack deploy \
    -c stacks/exporters-tutorial.yml \
    exporter
```

Please wait until the service in the stack are up-and-running. You can check their status by executing `docker stack ps exporter` command.

If you go back to Prometheus and navigate to the *Status > Configuration* screen, you'll notice that exporters are automatically added as well as the path to the rules file that contains alerts. To be on the safe side, please open the *Status > Targets* screen. It should contain three endpoints for each of the two targets we created.

The two alerts were created as well. You can see their status by navigating to the *Alerts* screen.

We'll deploy one more stack. This time, we'll create a few demo services as a way to demonstrate that alerts creation is not limited to exporters but that it can be applied to any Swarm service.

The stack we'll deploy is as follows.

```
version: '3'

services:

  main:
    image: vfarcic/go-demo
    environment:
      - DB=db
    ports:
      - 8080:8080
    deploy:
      replicas: 3
      update_config:
        parallelism: 1
        delay: 10s
      labels:
        - com.df.notify=true
        - com.df.distribute=true
        - com.df.alertName=memlimit
        - com.df.alertIf=@service_mem_limit:0.8
        - com.df.alertFor=30s
      resources:
        reservations:
          memory: 5M
        limits:
          memory: 10M

  db:
    image: mongo
```

In this context, the details of the services are not important. What matters is that we defined that the service should create an alert named `memlimit` and that the condition is defined as the `@service_mem_limit:0.8` shortcut. It will create an alert that will be fired if memory usage is over 80% of the memory limit which is set to 10MB. Additionally, we also set the `alertFor` label tells Prometheus to fire the alert only if the condition persists for more than 30 seconds.

Let's deploy the `go-demo` stack.

```bash
docker stack deploy \
    -c stacks/go-demo-tutorial.yml \
    go-demo
```

If you go back to the alerts screen, you'll see that a new entry is added.

It is up to you to configure [Alert Manager](https://hub.docker.com/r/prom/alertmanager/) so that those alerts are propagated accordingly (e.g. to Slack, Jenkins, email, and so on).

## What Now?

That was a very brief introduction to *Docker Flow Monitor*. Please consult the documentation for any additional information you might need. Feel free to open [an issue](https://github.com/vfarcic/docker-flow-monitor/issues) if you require additional info, if you find a bug, or if you have a feature request.

Before you go, please remove the cluster we created and free those resources for something else.

```bash
docker-machine rm -f swarm-1 swarm-2 swarm-3
```
