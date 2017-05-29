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
|alertFor       |This parameter is translated to Prometheus alert *FOR* statement. It causes Prometheus to wait for a certain duration between first encountering a new expression output vector element (like an instance with a high HTTP error rate) and counting an alert as firing for this element. Elements that are active, but not firing yet, are in pending state. This parameter expects a number with time suffix (e.g. `s` for seconds, `m` for minutes).<br>Example: `30s`, `5m`|No|
|alertIf        |This parameter is translated to Prometheus alert *IF* statement. It is an expression that will be evaluated and, if it returns *true*, an alert will be fired.<br>Example: `container_memory_usage_bytes{container_label_com_docker_swarm_service_name="go-demo"}/container_spec_memory_limit_bytes{container_label_com_docker_swarm_service_name="go-demo"} > 0.8`|Yes|
|alertName      |The name of the alert. It is combined with the `serviceName` thus producing an unique identifier.<br>Example: `memoryAlert`|Yes|
|serviceName    |The name of the service. It is combined with the `alertName` thus producing an unique identifier.<br>Example: `go-demo`|Yes|

Those parameters can be indexed so that multiple alerts can be defined for a service. Indexing is sequential and starts from 1. An example of indexed `alertName` could be `alertName.1=memload` and `alertName.2=diskload`.

Please visit [Alerting Overview](https://prometheus.io/docs/alerting/overview/) for more information about the rules for defining Prometheus alerts.

### AlertIf Parameter Shortcuts

> Allows short specification of commonly used `alertIf` parameters

|Shortcut|Description|
|@node_fs_limit:[PERCENTAGE]|Whether node file system usage is over specified percentage of the total available file system size.<br>Requirements: `node-exporter` metrics<br>[PERCENTAGE] must be specified as a decimal value (e.g. `0.8` equals `80%`).Example: `@node_fs_limit:0.8` would be expanded to `(node_filesystem_size{fstype="aufs"} - node_filesystem_free{fstype="aufs"}) / node_filesystem_size{fstype="aufs"} > 0.8`.|
|@node_mem_limit:[PERCENTAGE]|Whether node memory usage is over specified percentage of the total node memory.<br>Requirements: `node-exporter` metrics<br>[PERCENTAGE] must be specified as a decimal value (e.g. `0.8` equals `80%`).Example: `@node_mem_limit:0.8` would be expanded to `(sum by (instance) (node_memory_MemTotal) - sum by (instance) (node_memory_MemFree + node_memory_Buffers + node_memory_Cached)) / sum by (instance) (node_memory_MemTotal) > 0.8`.|
|@service_mem_limit:[PERCENTAGE]|Whether service memory usage is over specified percentage of the service memory limit.<br>Requirements: `cAdvisor` metrics and service memory limit specified as service resource.<br>[PERCENTAGE] must be specified as a decimal value (e.g. `0.8` equals `80%`).Example: If `serviceName` is set to `my-service`, `@service_mem_limit:0.8` would be expanded to `container_memory_usage_bytes{container_label_com_docker_swarm_service_name="my-service"}/container_spec_memory_limit_bytes{container_label_com_docker_swarm_service_name="my-service"} > 0.8`.|

## Remove

*Remove* endpoint can be used to send request to *Docker Flow Monitor* with the goal of removing scrapes and alerts related to a service.

Query parameters that follow should be added to the base address **[MONITOR_IP]:[MONITOR_PORT]/v1/docker-flow-monitor/remove**.

> Removes Prometheus scrapes and alerts

|Query          |Description                                                                               |Required|
|---------------|------------------------------------------------------------------------------------------|--------|
|serviceName    |The name of the service that should be removed.                                           |Yes     |