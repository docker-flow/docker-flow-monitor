# Auto-Scaling Services Using Instrumented Metrics

Docker Swarm provides a solid mechanism that, among other things, makes sure that the specified number of replicas of a service is (almost) always running inside a cluster. It is performing self-healing out-of-the-box. However, that is often not enough. We need the system to adapt to change conditions. We'll call this process self-adaptation.

In this tutorial, we'll go through one possible setup that allows self-adaptation of services based on their response time. That does not mean that response time metrics are the only ones we should use. Quite the contrary. However, we need to limit the scope of this tutorial and response times are probably one of the most commonly used metrics when applying self-adaptation.

The tools we'll use to setup a self-adaptive system are as follows.

* [Prometheus](https://prometheus.io/): Scrapes metrics and fires alerts when certain thresholds are reached.
* [Docker Flow Monitor](http://monitor.dockerflow.com/): It extends Prometheus with capability to auto-configure itself.
* [Alertmanager](https://prometheus.io/docs/alerting/alertmanager/): Receives alerts from Prometheus and forwards them to some other service depending on matching routes.
* [Jenkins](https://jenkins.io/): Executes scheduled or triggered jobs. We'll use it as the engine that will scale a service.
* [Docker Flow Proxy](http://proxy.dockerflow.com/): It extends HAProxy with capability to auto-configure itself.
* [Docker Flow Swarm Listener](http://swarmlistener.dockerflow.com/): Listens to Swarm events and sends notifications when a service is created or updated. We'll use it to send notifications to *Docker Flow Monitor* and *Docker Flow Proxy*.
* [go-demo](https://github.com/vfarcic/go-demo): A demo service.

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

## Deploying Docker Flow Proxy (DFP) and Docker Flow Swarm Listener (DFSL)

Proxy is not strictly necessary for this tutorial. We're using it only as a convenient way to get a single access point to the cluster instead opening a different port for each publicly accessible service.

```bash
docker network create -d overlay proxy

docker stack deploy \
    -c stacks/docker-flow-proxy-mem.yml \
    proxy
```

The stack deployed two services; `proxy` and `swarm-listener`. From now on, the proxy will be notified whenever a service is deployed or updated as long as it has the `com.df.notify` label set to `true`. Please consult [docker-flow-proxy-mem.yml](https://github.com/vfarcic/docker-flow-monitor/blob/master/stacks/docker-flow-proxy-mem.yml) for the full definition of the stack. For information about those two projects, please visit [proxy.dockerflow.com](http://proxy.dockerflow.com) and [swarmlistener.dockerflow.com](http://swarmlistener.dockerflow.com/).

## Deploying Docker Flow Monitor and Alertmanager

The next stack defines *Docker Flow Monitor* and *Alertmanager*. Before we deploy the stack, we should create the `monitor` network that will allow Prometheus to scrape metrircs from exporters and instrumented services.

```bash
docker network create -d overlay monitor
```

Next we'll create *Alertmanager* configuration as a Docker secret. That way we won't need to create a new image with configuration or mount a volume.

```bash
echo "route:
  group_by: [service,scale]
  repeat_interval: 5m
  group_interval: 5m
  receiver: 'slack'
  routes:
  - match:
      service: 'go-demo_main'
      scale: 'up'
    receiver: 'jenkins-go-demo_main-up'
  - match:
      service: 'go-demo_main'
      scale: 'down'
    receiver: 'jenkins-go-demo_main-down'

receivers:
  - name: 'slack'
    slack_configs:
      - send_resolved: true
        title: '[{{ .Status | toUpper }}] {{ .GroupLabels.service }} service is in danger!'
        title_link: 'http://$(docker-machine ip swarm-1)/monitor/alerts'
        text: '{{ .CommonAnnotations.summary}}'
        api_url: 'https://hooks.slack.com/services/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'
  - name: 'jenkins-go-demo_main-up'
    webhook_configs:
      - send_resolved: false
        url: 'http://$(docker-machine ip swarm-1)/jenkins/job/service-scale/buildWithParameters?token=DevOps22&service=go-demo_main&scale=1'
  - name: 'jenkins-go-demo_main-down'
    webhook_configs:
      - send_resolved: false
        url: 'http://$(docker-machine ip swarm-1)/jenkins/job/service-scale/buildWithParameters?token=DevOps22&service=go-demo_main&scale=-1'
" | docker secret create alert_manager_config -
```

The configuration groups routes by `service` and `scale` labels. The `repeat_interval` and `group_interval` are both set to five minutes. In a production cluster, `repeat_interval` should be set to a much larger value (e.g. `1h`). We set it up to five minutes so that we can demonstrate different features of the system faster. Otherwise, we'd need to wait for over an hour to see different alerts in action.

The default receiver is `slack`. As the result, any alert that does not match one of the `routes` will be sent to Slack.

The `routes` section defines two `match` entries. If the alert label `service` is set to `go-demo_main` and the label `scale` is `up`, the receiver will be `jenkins-go-demo_main-up`. Similarly, when the same service is associated with an alert but the `scale` label is set to `down`, the receiver will be `jenkins-go-demo_main-down`.

There are three receivers. The `slack` receiver will send notifications to Slack. As stated before, it's used only for alerts that do not `match` one of the `routes`. Both `jenkins-go-demo_main-up` and `jenkins-go-demo_main-down` are sending a `POST` request to Jenkins job `service-scale`. The only difference between the two is in the `scale` parameter. One will set it to `1` indicating that the `go-demo_main` service should be up-scaled by one and the other will set it to `-1` indicating that the service should de de-scaled by 1.

Please consult [configuration](https://prometheus.io/docs/alerting/configuration/) section of Alertmanager documentation for more information about the options we used.

Now we can deploy the `monitor` stack.

```bash
DOMAIN=$(docker-machine ip swarm-1) \
    docker stack deploy \
    -c stacks/docker-flow-monitor-slack.yml \
    monitor
```

The full definition of the stack that we just deployed can be found in [docker-flow-monitor-slack.yml](https://github.com/vfarcic/docker-flow-monitor/blob/master/stacks/docker-flow-monitor-slack.yml). We'll comment only on a few interesting parts. The definition, limited to relevant parts, is as follows.

```
...
  monitor:
    image: vfarcic/docker-flow-monitor
    environment:
      - LISTENER_ADDRESS=swarm-listener
      - GLOBAL_SCRAPE_INTERVAL=${SCRAPE_INTERVAL:-10s}
      - ARG_WEB_ROUTE-PREFIX=/monitor
      - ARG_WEB_EXTERNAL-URL=http://${DOMAIN:-localhost}/monitor
      - ARG_ALERTMANAGER_URL=http://alert-manager:9093
    ...
    deploy:
      labels:
        ...
        - com.df.servicePath=/monitor
        - com.df.serviceDomain=${DOMAIN:-localhost}
        - com.df.port=9090
      ...

  alert-manager:
    image: prom/alertmanager
    networks:
      - monitor
    secrets:
      - alert_manager_config
    command: -config.file=/run/secrets/alert_manager_config -storage.path=/alertmanager

  swarm-listener:
    image: vfarcic/docker-flow-swarm-listener
    ...
    environment:
      - DF_NOTIFY_CREATE_SERVICE_URL=http://monitor:8080/v1/docker-flow-monitor/reconfigure
      - DF_NOTIFY_REMOVE_SERVICE_URL=http://monitor:8080/v1/docker-flow-monitor/remove
    ...
```

Inside the `monitor` service, we used environment variables to provide initial Prometheus configuration. The labels will be used by `swarm` listener to notify the proxy about monitor's path, domain, and port.

The `alert-manager` service uses `alert_manager_config` secret as Alertmanager configuration file.

The `swarm-listener` service has `monitor` running on port `8080` as URL where notifications should be sent.

Please consult [Docker Flow Monitor documentation](http://monitor.dockerflow.com/) for more information. If you haven't used it before, the [Running Docker Flow Monitor tutorial](http://monitor.dockerflow.com/tutorial/) might be a good starting point.

Let us confirm that the `monitor` stack is up and running.

```bash
docker stack ps monitor
```

Please wait a few moments if some of the replicas do not yet have the status set to `running`.

Now that the `monitor` stack is up and running, we should proceed with deployment of Jenkins and its agent.

## Deploying Jenkins

The Jenkins image we'll run already has all the plugins baked in. The administrative user and password will be retrieved from Docker secrets. A job that will scale and de-scale services is also defined inside the image. With those in place, we'll be able to skip manual setup.

```bash
echo "admin" | \
    docker secret create jenkins-user -

echo "admin" | \
    docker secret create jenkins-pass -

export SLACK_IP=$(ping \
    -c 1 devops20.slack.com \
    | awk -F'[()]' '/PING/{print $2}')

docker stack deploy \
    -c stacks/jenkins-scale.yml jenkins
```

We created two secrets that define administrative username and password. The environment variable `SLACK_IP` might not be necessary. It's there just in case Docker Machine cannot resolve Slack. Finally, the last command deployed the `jenkins` stack.

I won't go into much detailed about the `jenkins` stack. If you're interested in the subject, you might want to read [Automating Jenkins Docker Setup](https://technologyconversations.com/2017/06/16/automating-jenkins-docker-setup/) article or watch the [Jenkins Master As a Docker Service Running Inside a Docker for AWS Cluster](https://technologyconversations.com/2017/08/03/jenkins-master-as-a-docker-service-running-inside-a-docker-for-aws-cluster/) video. The only thing that truly matters is the `service-scale` job that we'll explore soon.

Before we proceed, please confirm that all the replicas of the stack are running.


```bash
docker stack ps jenkins
```

Once all the replicas of the stack are in the `running` state, we can open the `service-scale` job and take a quick look at its definition.

> If you're a Windows user, Git Bash might not be able to use the `open` command. If that's the case, replace the `open` command with `echo`. As a result, you'll get the full address that should be opened directly in your browser of choice.

```bash
open "http://$(docker-machine ip swarm-1)/jenkins/job/service-scale/configure"
```

You will be presented with a login screen. Please use `admin` as both username and password to authenticate.

Please click the *Pipeline* tab once you get inside the `service-scale` configuration screen.

The first half of the job is relatively straightforward. The job should be executed inside a `prod` agent (short for production). It defines two parameters. One holds the name of the service that should be scaled. The other expected a number of replicas that should be added or removed. If the value is positive, the service will be up-scaled. A negative value means that it should de-scale.

The job defines only one stage called `Scale`. Inside it is a single step defined inside a `script`. It executes `docker service inspect` command and retrieves the current number of replicas. It also retrieves `scaleMin` and `scaleMax` labels to discover the limits that should be applied to scaling. Without them, we would run a risk of scaling to infinity or de-scaling to zero replicas.

The desired number of replicas (`newReplicas`) is obtained by subtracting the current number of replicas with the `scale` parameter.

Once all the variables are set, it evaluates whether scaling would hit thresholds defined with `scaleMin` and `scaleMax`. If it would, it throws an error which, later on in the `post` section, results in a message to Slack. If neither thresholds would be reached, a simple `docker service scale` command is executed.

Since Jenkins pipeline is defined using [Declarative syntax](https://jenkins.io/doc/book/pipeline/syntax/#declarative-pipeline), the first execution needs to be manual so that it is correctly processed and the parameters are created.

Please open the `service-scale` activity screen.

```bash
open "http://$(docker-machine ip swarm-1)/jenkins/blue/organizations/jenkins/service-scale/activity"
```

Now click the *Run* button. A few moments later, you'll see that the build failed. Don't panic. That is expected. It's a workaround to bypass a bug and create the proper job definition with all the parameters. It will not fail again for the same reason.

## Deploying Instrumented Service

The [go-demo](https://github.com/vfarcic/go-demo) service is already instrumented. Among others, it request generates `resp_time` metrics with response time, service name, response code, and path labels.

We won't go into details of how the service was instrumented but only comment on a few snippets. The code of the whole service is in a single file [main.go](https://github.com/vfarcic/go-demo/blob/master/main.go). Do not be afraid! We're using Go only to demonstrate how instrumentation works. You can implement similar principles in almost any programming language. Hopefully, you should have no problem understanding the logic behind it even if Go is not your programming language of choice.

As an example, every request starting with the `/demo/hello` path is sent to the `HelloServer` function. The relevant part of the function is as follows.

```go
func HelloServer(w http.ResponseWriter, req *http.Request) {
	start := time.Now()
	defer func() { recordMetrics(start, req, http.StatusOK) }()
	...
}
```

We record the time (`start`) and defer the invocation of the `recordMetric` function. In Go, `defer` means that the function will be executed at the end of the context it is defined in. Like that, we guarantee that the `recordMetrics` will be invoked after the request is processed and the response is sent back to the client.

The `recordMetric` function records (observes) the duration of the response by calculating the difference between the current and the start time. That observation is done with a few labels that will, later on, allow us to query metrics from Prometheus and define alerts.

```go
func recordMetrics(start time.Time, req *http.Request, code int) {
	duration := time.Since(start)
	histogram.With(
		prometheus.Labels{
			"service": serviceName,
			"code": fmt.Sprintf("%d", code),
			"method": req.Method,
			"path": req.URL.Path,
		},
	).Observe(duration.Seconds())
}
```

For more information, please consult [instrumentation](https://prometheus.io/docs/practices/instrumentation/) or [client libraries](https://prometheus.io/docs/instrumenting/clientlibs/) pages of Prometheus documentation.

Now we can deploy the last stack. It will be the service we're hoping to scale based on response time metrics.

```bash
docker stack deploy \
    -c stacks/go-demo-instrument-alert-short.yml \
    go-demo
```

Please visit [go-demo-instrument-alert-short.yml](https://github.com/vfarcic/docker-flow-monitor/blob/master/stacks/go-demo-instrument-alert-short.yml) for the full stack definition. We'll comment only on service labels since the rest should be pretty straightforward.


```
  main:
    ...
    deploy:
      ...
      labels:
        - com.df.notify=true
        - com.df.distribute=true
        - com.df.servicePath=/demo
        - com.df.port=8080
        - com.df.scaleMin=2
        - com.df.scaleMax=4
        - com.df.scrapePort=8080
        - com.df.alertName.1=memlimit
        - com.df.alertIf.1=@service_mem_limit:0.8
        - com.df.alertFor.1=5m
        - com.df.alertName.2=resptimeabove
        - com.df.alertIf.2=@resp_time_above:0.1,5m,0.99
        - com.df.alertName.3=resptimebelow
        - com.df.alertIf.3=@resp_time_below:0.025,5m,0.75
      ...
```

The `servicePath` and `port` label will be used by *Docker Flow Proxy* to configure itself and start forwarding requests coming to `/demo` to the `go-demo` service.

You already saw the usage of `scaleMin` and `scaleMax` labels. Jenkins uses them to decide whether the service should be scale or the number of replicas already reached the limits.

The `alertName`, `alertIf`, and `alertFor` labels are the key to scaling. The define Prometheus alerts. The first one (`memlimit`) is already described in the [Running Docker Flow Monitor tutorial](http://monitor.dockerflow.com/tutorial/) so will skip it. The second (`resptimeabove`) defines alert that will be fired if the rate of response times of the `0.1` seconds bucket (100 milliseconds or faster) is above 99% (`0.99`) for over five minutes (`5m`). Similarly, the `resptimebelow` alert will fire if the rate of response times of the `0.025` seconds bucket (25 milliseconds or faster) is below 75% (`0.75`) for over five minutes (`5m`). In all the cases, we're using [AlertIf Parameter Shortcuts](http://monitor.dockerflow.com/usage/#alertif-parameter-shortcuts) that will be expanded into full Prometheus expressions.

Let's take a look at Prometheus alert screen.

```bash
open "http://$(docker-machine ip swarm-1)/monitor/alerts"
```

You should see three alerts that correspond to the three labels define in the `main` service of the `go-demo` stack. *Docker Flow Swarm Listener* detected the new service and sent those labels to *Docker Flow Monitor* which, in turn, converted them info Prometheus configuration.

If you expand the *godemomainresptimeabove* alert, you'll see that DFM translated the service labels into the alert definition that follows.

```
ALERT godemomainresptimeabove
  IF sum(rate(http_server_resp_time_bucket{job="go-demo_main",le="0.1"}[5m])) / sum(rate(http_server_resp_time_count{job="go-demo_main"}[5m])) < 0.99
  LABELS {receiver="system", scale="up", service="go-demo_main"}
  ANNOTATIONS {summary="Response time of the service go-demo_main is above 0.1"}
```

Similarly, the *godemomainresptimebelow* alert is defined as follows.

```
ALERT godemomainresptimebelow
  IF sum(rate(http_server_resp_time_bucket{job="go-demo_main",le="0.025"}[5m])) / sum(rate(http_server_resp_time_count{job="go-demo_main"}[5m])) > 0.75
  LABELS {receiver="system", scale="down", service="go-demo_main"}
  ANNOTATIONS {summary="Response time of the service go-demo_main is below 0.025"}
```

Let's confirm that the `go-demo` stack is up-and-running.

```bash
docker stack ps -f desired-state=running go-demo
```

You should see three replicas of the `go-demo_main` and one replica of the `go-demo_db` service. If that's not the case, please wait a while longer and repeat the `docker stack ps` command.

We should confirm that all the targets of the service are indeed registered.

```bash
open "http://$(docker-machine ip swarm-1)/monitor/targets"
```

You should see two or three targets depending on whether Prometheus already sent the alert to de-scale the service (more on that soon).

## Automatically Scaling Services

Let's go back to the Prometheus' alert screen.

```bash
open "http://$(docker-machine ip swarm-1)/monitor/alerts"
```

By this time, the *godemomainresptimebelow* alert should be red. The `go-demo` service periodically pings itself and the response is faster than the twenty-five milliseconds limit we set (unless your laptop is very old and slow). As a result, Prometheus fired the alert to Alertmanager. It, in turn, evaluated the `service` and `scale` labels and decided that it should send a POST request to Jenkins with parameters `service=go-demo_main&scale=-1`.

We can confirm that the process worked by opening the Jenkins `service-scale` activity screen.

```bash
open "http://$(docker-machine ip swarm-1)/jenkins/blue/organizations/jenkins/service-scale/activity"
```

You should see that the new build was executed and, hopefully, it's green. If more than ten minutes passed, you might see a third build as well. If that's the case, we'll ignore it for now.

Please click the second (green) build followed with a click to the last step with the name *Print Message*. The output should say that *go-demo_main was scaled from 3 to 2 replicas*.

Let's double check that's what truly happened.

```bash
docker stack ps -f desired-state=running go-demo
```

The output should be similar to the one that follows (IDs are removed for brevity).

```
NAME           IMAGE                  NODE    DESIRED STATE CURRENT STATE         ERROR PORTS
go-demo_main.1 vfarcic/go-demo:latest swarm-2 Running       Running 2 minutes ago
go-demo_db.1   mongo:latest           swarm-2 Running       Running 3 minutes ago
go-demo_main.2 vfarcic/go-demo:latest swarm-3 Running       Running 2 minutes ago
```

As you can see, Prometheus used metrics to deduce that we have more replicas in the system than we really need since they respond very fast. As a result, if fired an alert to Alertmanager which executed a Jenkins build and our service was scaled down from three to two replicas.

If you take a closer look at the Alertmanager configuration, you'll notice that both the `repeat_interval` and the `group_interval` are set to five minutes. If Prometheus continues firing the alert, Alertmanager will repeat the same process ten minutes later.

Please observe the Jenkins `service-scale` screen. Ten minutes later a new build will start. However, since we are already running the minimum number of replicas, Jenkins will send a notification to Slack instead trying to continue de-scaling the service.

Please visit the *#df-monitor-tests* channel inside [devops20.slack.com](https://devops20.slack.com/) and you should see a Slack notification stating that *go-demo_main could not be scaled*. If this is your first visit to *devops20* on Slack, you'll have to register through [slack.devops20toolkit.com](http://slack.devops20toolkit.com).

Let's see what happens when response times of the service become too high. We'll send requests that will result in high response time and observe the behavior of the system.

```bash
for i in {1..30}; do
    DELAY=$[ $RANDOM % 6000 ]
    curl "http://$(docker-machine ip swarm-1)/demo/hello?delay=$DELAY"
done
```

If the service receives the `delay` parameter, it goes to sleep for the specified number of milliseconds. The above commands sent thirty requests with a random delay between 0 and 6000 milliseconds.

Now we can take a look at the alerts.

```bash
open "http://$(docker-machine ip swarm-1)/monitor/alerts"
```

The *godemomainresptimeabove* turned red indicating that the threshold is reached and Prometheus fired an alert to Alertmanager. If everything went as planned, Alertmanager should have sent a request to Jenkins. Let's confirm that indeed happened.

```bash
open "http://$(docker-machine ip swarm-1)/jenkins/blue/organizations/jenkins/service-scale/activity"
```

You should see a new build. Please click it. The last step with the *Print Message* header should state that *go-demo_main was scaled from 2 to 3 replicas*.

We can confirm that the number of replicas indeed scaled to three by taking a look at the stack processes.

```bash
docker stack ps -f desired-state=running go-demo
```

The output should be similar to the one that follows (IDs are removed for brevity).

```
NAME           IMAGE                  NODE    DESIRED STATE CURRENT STATE             ERROR PORTS
go-demo_main.1 vfarcic/go-demo:latest swarm-2 Running       Running about an hour ago
go-demo_db.1   mongo:latest           swarm-2 Running       Running about an hour ago
go-demo_main.2 vfarcic/go-demo:latest swarm-3 Running       Running about an hour ago
go-demo_main.3 vfarcic/go-demo:latest swarm-1 Running       Running 42 seconds ago
```

## What Now?

You saw a simple example of a system that automatically scales and de-scales services. You should be able to expand on those examples and start building your own self-sufficient system that features not only self-healing provided with Docker Swarm but also self-adaptation based on scraped metrics.

Please remove the demo cluster we created and free your resources.

```bash
docker-machine rm -f swarm-1 swarm-2 swarm-3
```

## The DevOps 2.2 Toolkit: Self-Healing Docker Clusters

The article you just read uses some of the concepts and exercises described in [The DevOps 2.2 Toolkit: Self-Healing Docker Clusters](https://leanpub.com/the-devops-2-2-toolkit).

<a href="https://leanpub.com/the-devops-2-2-toolkit"><img src="https://technologyconversations.files.wordpress.com/2017/06/cover-small1.jpg?w=249" alt="" width="249" height="300" class="alignright size-medium wp-image-3563" /></a>If you liked this article, you might be interested in **[The DevOps 2.2 Toolkit: Self-Healing Docker Clusters](https://leanpub.com/the-devops-2-2-toolkit)** book. The book goes beyond Docker and schedulers and tries to explore ways for building self-adaptive and self-healing Docker clusters. If you are a Docker user and want to explore advanced techniques for creating clusters and managing services, this book might be just what you're looking for.

The book is still under development. If you choose to become an early reader and influence the direction of the book, please get a copy from [LeanPub](https://leanpub.com/the-devops-2-2-toolkit). You will receive notifications whenever a new chapter is added.

Give the book a try and let me know what you think.