# Usage

*Docker Flow Monitor* can be controlled by sending HTTP requests or through Docker Service labels when combined with [Docker Flow Swarm Listener](http://swarmlistener.dockerflow.com/).

## Reconfigure

*Reconfigure* endpoint can be used to send requests to *Docker Flow Monitor* with the goal of adding or modifying existing scrape targets and alerts. Parameters are divided into *scrape* and *alert* groups.

Query parameters that follow should be added to the base address **[MONITOR_IP]:[MONITOR_PORT]/v1/docker-flow-monitor/reconfigure**.

### Scrape Parameters

!!! tip
    Defines Prometheus scrape targets

|Query          |Description                                                                               |Required|
|---------------|------------------------------------------------------------------------------------------|--------|
|metricsPath    |The path of the metrics endpoint. Defaults to `/metrics`.                                 |No      |
|scrapeInterval |How frequently to scrape targets from this job.                                           |No      |
|scrapeTimeout  |Per-scrape timeout when scraping this job.                                                |No      |
|scrapePort     |The port through which metrics are exposed.                                               |Yes     |
|serviceName    |The name of the service that exports metrics.                                             |Yes     |
|scrapeType     |A set of targets and parameters describing how to scrape metrics.                         |No      |

You can find more about scrapeType's on [Scrape Config](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#scrape_config).

### Alert Parameters

!!! tip
    Defines Prometheus alerts

|Query          |Description                                                                               |Required|
|---------------|------------------------------------------------------------------------------------------|--------|
|alertAnnotations|This parameter is translated to Prometheus alert `ANNOTATIONS` statement. Annotations are used to store longer additional information.<br>**Example:** `summary=Service memory is high,description=Do something or start panicking`|No|
|alertFor       |This parameter is translated to Prometheus alert `FOR` statement. It causes Prometheus to wait for a certain duration between first encountering a new expression output vector element (like an instance with a high HTTP error rate) and counting an alert as firing for this element. Elements that are active, but not firing yet, are in pending state. This parameter expects a number with time suffix (e.g. `s` for seconds, `m` for minutes).<br>**Example:** `30s`|No|
|alertIf        |This parameter is translated to Prometheus alert `IF` statement. It is an expression that will be evaluated and, if it returns *true*, an alert will be fired.<br>Example: `container_memory_usage_bytes{container_label_com_docker_swarm_service_name="go-demo"}/container_spec_memory_limit_bytes{container_label_com_docker_swarm_service_name="go-demo"} > 0.8`|Yes|
|alertLabels    |This parameter is translated to Prometheus alert `LABELS` statement. It allows specifying a set of additional labels to be attached to the alert. Multiple labels can be separated with comma (`,`).<br>**Example:** `severity=high,receiver=system`|No|
|alertName      |The name of the alert. It is combined with the `serviceName` thus producing an unique identifier.<br>**Example:** `memoryAlert`|Yes|
|serviceName    |The name of the service. It is combined with the `alertName` thus producing an unique identifier.<br>**Example:** `go-demo`|Yes|

Those parameters can be indexed so that multiple alerts can be defined for a service. Indexing is sequential and starts from 1. An example of indexed `alertName` could be `alertName.1=memload` and `alertName.2=diskload`.

Please visit [Alerting Overview](https://prometheus.io/docs/alerting/overview/) for more information about the rules for defining Prometheus alerts.

### AlertIf Parameter Shortcuts

!!! tip
    Allows short specification of commonly used `alertIf` parameters

|Shortcut                          |Description                                                    |
|----------------------------------|---------------------------------------------------------------|
|@node_fs_limit:[PERCENTAGE]       |Whether node file system usage is over specified percentage of the total available file system size.<br>**Requirements:** `node-exporter` metrics<br>[PERCENTAGE] must be specified as a decimal value (e.g. `0.8` equals `80%`).<br>**Example:** `@node_fs_limit:0.8` would be expanded to `(node_filesystem_size{fstype="aufs", job="my-service"} - node_filesystem_free{fstype="aufs", job="my-service"}) / node_filesystem_size{fstype="aufs", job="my-service"} > 0.8`.|
|@node_mem_limit:[PERCENTAGE]      |Whether node memory usage is over specified percentage of the total node memory.<br>**Requirements:** `node-exporter` metrics<br>[PERCENTAGE] must be specified as a decimal value (e.g. `0.8` equals `80%`).<br>**Example:** `@node_mem_limit:0.8` would be expanded to `(sum by (instance) (node_memory_MemTotal{job="my-service"}) - sum by (instance) (node_memory_MemFree{job="my-service"} + node_memory_Buffers{job="my-service"} + node_memory_Cached{job="my-service"})) / sum by (instance) (node_memory_MemTotal{job="my-service"}) > 0.8`.|
|@node_mem_limit_total_above:[PERCENTAGE]|Whether memory usage of all the nodes is over the specified percentage of the total memory.<br>**Requirements:** `node-exporter` metrics<br>[PERCENTAGE] must be specified as a decimal value (e.g. `0.8` equals `80%`).<br>**Example:** `@node_mem_limit_total_above:0.8` would be expanded to `(sum(node_memory_MemTotal{job="my-service"}) - sum(node_memory_MemFree{job="my-service"} + node_memory_Buffers{job="my-service"} + node_memory_Cached{job="my-service"})) / sum(node_memory_MemTotal{job="my-service"}) > 0.8`.|
|@node_mem_limit_total_below:[PERCENTAGE]|Whether memory usage of all the nodes is below the specified percentage of the total memory.<br>**Requirements:** `node-exporter` metrics<br>[PERCENTAGE] must be specified as a decimal value (e.g. `0.8` equals `80%`).<br>**Example:** `@node_mem_limit_total_below:0.4` would be expanded to `(sum(node_memory_MemTotal{job="my-service"}) - sum(node_memory_MemFree{job="my-service"} + node_memory_Buffers{job="my-service"} + node_memory_Cached{job="my-service"})) / sum(node_memory_MemTotal{job="my-service"}) < 0.4`.|
|@replicas_running|Whether the number of running replicas is as desired.<br>**Requirements:** `cAdvisor` metrics and a service running in the replicated mode. The alert uses `container_memory_usage_bytes` metric only as a way to count the number of running containers.<br>**Example:** `@replicas_running` for a service with the number of desired replicas set to `3` would be expanded to `count(container_memory_usage_bytes{container_label_com_docker_swarm_service_name="my-service"}) != 3`.|
|@replicas_more_than|Whether the number of running replicas is more than desired.<br>**Requirements:** `cAdvisor` metrics and a service running in the replicated mode. The alert uses `container_memory_usage_bytes` metric only as a way to count the number of running containers.<br>**Example:** `@replicas_running` for a service with the number of desired replicas set to `3` would be expanded to `count(container_memory_usage_bytes{container_label_com_docker_swarm_service_name="my-service"}) > 3`.|
|@replicas_less_than|Whether the number of running replicas is less than desired.<br>**Requirements:** `cAdvisor` metrics and a service running in the replicated mode. The alert uses `container_memory_usage_bytes` metric only as a way to count the number of running containers.<br>**Example:** `@replicas_running` for a service with the number of desired replicas set to `3` would be expanded to `count(container_memory_usage_bytes{container_label_com_docker_swarm_service_name="my-service"}) < 3`.|
|@resp_time_above:[QUANTILE],[RATE_DURATION],[PERCENTAGE]|Whether response time of a given *quantile* over the specified *rate duration* is above the set *percentage*.<br>**Requirements:** histogram with the name `http_server_resp_time` and with response times expessed in seconds.<br>[QUANTILE] must be one of the quantiles defined in the metric.<br>[RATE_DURATION] can be in any format supported by Prometheus (e.g. `5m`).<br>[PERCENTAGE] must be specified as a decimal value (e.g. `0.8` equals `80%`).<br>**Example:** `@resp_time_above:0.1,5m,0.9999` would be expanded to `sum(rate(http_server_resp_time_bucket{job="my-service", le="0.1"}[5m])) / sum(rate(http_server_resp_time_count{job="my-service"}[5m])) < 0.9999`.|
|@resp_time_below:[QUANTILE],[RATE_DURATION],[PERCENTAGE]|Whether response time of a given *quantile* over the specified *rate duration* is below the set *percentage*.<br>**Requirements:** histogram with the name `http_server_resp_time` and with response times expessed in seconds.<br>[QUANTILE] must be one of the quantiles defined in the metric.<br>[RATE_DURATION] can be in any format supported by Prometheus (e.g. `5m`).<br>[PERCENTAGE] must be specified as a decimal value (e.g. `0.8` equals `80%`).<br>**Example:** `@resp_time_below:0.025,5m,0.75` would be expanded to `sum(rate(http_server_resp_time_bucket{job="my-service", le="0.025"}[5m])) / sum(rate(http_server_resp_time_count{job="my-service"}[5m])) > 0.75`.|
|@resp_time_server_error:[RATE_DURATION],[PERCENTAGE]|Whether error rate over the specified *rate duration* is below the set *percentage*.<br>**Requirements:** histogram with the name `http_server_resp_time` and with label `code` set to value of the HTTP response code.<br>[RATE_DURATION] can be in any format supported by Prometheus (e.g. `5m`).<br>[PERCENTAGE] must be specified as a decimal value (e.g. `0.8` equals `80%`).<br>**Example:** `@resp_time_server_error:5m,0.001` would be expanded to `sum(rate(http_server_resp_time_count{job="my-service", code=~"^5..$$"}[5m])) / sum(rate(http_server_resp_time_count{job="my-service"}[5m])) > 0.001`.|
|@service_mem_limit:[PERCENTAGE]   |Whether service memory usage is over specified percentage of the service memory limit.<br>**Requirements:** `cAdvisor` metrics and service memory limit specified as service resource.<br>[PERCENTAGE] must be specified as a decimal value (e.g. `0.8` equals `80%`).<br>**Example:** If `serviceName` is set to `my-service`, `@service_mem_limit:0.8` would be expanded to `container_memory_usage_bytes{container_label_com_docker_swarm_service_name="my-service"}/container_spec_memory_limit_bytes{container_label_com_docker_swarm_service_name="my-service"} > 0.8`.|

!!! note
    I hope that the number of shortcuts will grow with time thanks to community contributions. Please create [an issue](https://github.com/vfarcic/docker-flow-monitor/issues) with the `alertIf` statement and the suggested shortcut and I'll add it to the code as soon as possible.

### AlertIf Secrets Configuration

*Docker Flow Monitor* supports [Docker Secrets](https://docs.docker.com/engine/swarm/secrets/) for adding custom alertIf shortcuts. Only secrets with names that start with `alertif-` or `alertif_` will be considered. `alertIf` shortcuts are configured as a yaml file with a series of dictionaries. The key of each dictionary is your custom `alertIf` shortcut which must begin with the `@` character. The value of each dictionary consist of three keys: `expanded`, `annotations` and `labels`. `expanded` contains the expanded alert using go [templates](https://golang.org/pkg/text/template/). `annotations` and `labels` contains a dictionary with the alert's annotations and labels. For example `@service_mem_limit` is defined by the following yaml:

```yaml
"@service_mem_limit":
  expanded: container_memory_usage_bytes{container_label_com_docker_swarm_service_name="{{ .Alert.ServiceName }}"}/container_spec_memory_limit_bytes{container_label_com_docker_swarm_service_name="{{ .Alert.ServiceName }}"} > {{ index .Values 0 }}
  annotations:
    summary: Memory of the service {{ .Alert.ServiceName }} is over {{ index .Values 0 }}
  labels:
    receiver: system
    service: "{{ .Alert.ServiceName }}"
```

!!! tip
    AlertIf shortcuts defined in secrets will take priority over default shortcuts.

### AlertIf Logical Operators

The logical operators `and`, `unless`, and `or` can be used in combinations with AlertIf Parameter Shortcuts. For example, to create an alert that triggers when response time is low unless response time is high, set `alertIf=@resp_time_below:0.025,5m,0.75_unless_@resp_time_above:0.1,5m,0.99`. This alert prevents `@resp_time_below` from triggering while `@resp_time_above` is triggering. The `summary` annotation for this alert will be merged with the `and` operator: "Response time of the service my-service is below 0.025 unless Response time of the service my-service is above 0.1". When using logical operators, there are no default alert labels. The alert labels will have to be manually set by using the `alertLabels` query parameter.

 More information on the logical operators can be found on Prometheus's querying [documentation](https://prometheus.io/docs/prometheus/latest/querying/operators/#logical-set-binary-operators).

## Remove

!!! tip
    Removes Prometheus scrapes and alerts

*Remove* endpoint can be used to send request to *Docker Flow Monitor* with the goal of removing scrapes and alerts related to a service.

Query parameters that follow should be added to the base address **[MONITOR_IP]:[MONITOR_PORT]/v1/docker-flow-monitor/remove**.

|Query          |Description                                                                               |Required|
|---------------|------------------------------------------------------------------------------------------|--------|
|serviceName    |The name of the service that should be removed.                                           |Yes     |
