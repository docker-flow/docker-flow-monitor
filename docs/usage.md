# Usage

*Docker Flow Monitor* can be controlled by sending HTTP requests or through Docker Service labels when combined with [Docker Flow Swarm Listener](http://swarmlistener.dockerflow.com/).

## Reconfigure

*Reconfigure* endpoint can be used to send requests to *Docker Flow Monitor* with the goal of adding or modifying existing scrape targets and alerts. Parameters are divided into *scrape* and *alert* groups.

Query parameters that follow should be added to the base address **[MONITOR_IP]:[MONITOR_PORT]/v1/docker-flow-monitor/reconfigure**.

### Scrape Parameters

> Defines Prometheus scrape targets

|Query          |Description                                                                               |Required|
|---------------|------------------------------------------------------------------------------------------|--------|
|scrapePort     |The port through which metrics are exposed.                                               |Yes     |
|serviceName    |The name of the service that exports metrics.                                             |Yes     |

### Alert Parameters

> Defines Prometheus alerts

|Query          |Description                                                                               |Required|
|---------------|------------------------------------------------------------------------------------------|--------|
|alertFor       |This parameter is translated to Prometheus alert *FOR* statement. It causes Prometheus to wait for a certain duration between first encountering a new expression output vector element (like an instance with a high HTTP error rate) and counting an alert as firing for this element. Elements that are active, but not firing yet, are in pending state.|No|
|alertIf        |This parameter is translated to Prometheus alert *IF* statement. It is an expression that will be evaluated and, if it returns *true*, an alert will be fired.|Yes|
|alertName      |The name of the alert. It is combined with the `serviceName` thus producing an unique identifier.|Yes|
|serviceName    |The name of the service. It is combined with the `alertName` thus producing an unique identifier.|Yes|

Please visit [Alerting Overview](https://prometheus.io/docs/alerting/overview/) for more information about the rules for defining Prometheus alerts.

## Remove

*Remove* endpoint can be used to send request to *Docker Flow Monitor* with the goal of removing scrapes and alerts related to a service.

Query parameters that follow should be added to the base address **[MONITOR_IP]:[MONITOR_PORT]/v1/docker-flow-monitor/remove**.

> Removes Prometheus scrapes and alerts

|Query          |Description                                                                               |Required|
|---------------|------------------------------------------------------------------------------------------|--------|
|serviceName    |The name of the service that should be removed.                                           |Yes     |