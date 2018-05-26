# Configuring Docker Flow Monitor

*Docker Flow Monitor* can be configured through Docker environment variables and/or by creating a new image based on `vfarcic/docker-flow-monitor`.

## Startup Arguments

Environment variables prefixed will `ARG_` are used instead Prometheus startup arguments.

The formatting rules for the `ARG` variables are as follows:

1. Variable name has to be prefixed with `ARG_`.
2. Underscores (`_`) will be replaced with dots (`.`).
3. Capital letters will be transformed to lower case.

For example, if environment variables `ARG_WEB_ROUTE-PREFIX=/monitor` and `ARG_WEB_EXTERNAL-URL=http://localhost/monitor` are defined, Prometheus will be started with the arguments `web.route-prefix=/monitor` and `web.external-url=http://localhost/monitor`. The result would be Prometheus initialization equivalent to the command that follows.

```bash
prometheus --web.route-prefix=/monitor --web.external-url=http://localhost/monitor
```

`ARG` variables defined by default are as follows.

```
ARG_CONFIG_FILE=/etc/prometheus/prometheus.yml
ARG_STORAGE_TSDB_PATH=/prometheus
ARG_WEB_CONSOLE_LIBRARIES=/usr/share/prometheus/console_libraries
ARG_WEB_CONSOLE_TEMPLATES=/usr/share/prometheus/consoles
```

The `ARG_ALERTMANAGER_URL` environment variable accepts a comma separated list of alertmanagers:

```
ARG_ALERTMANAGER_URL=http://alert-manager:9093,http://alert-manager2:9093
```

These urls are configured in Prometheus's config file as `static_configs`:

```yaml
alerting:
  alertmanagers:
  - static_configs:
    - targets:
      - alert-manager:9093
      - alert-manager2:9093
    scheme: http
```

## Configuration

Environment variables prefixed with `GLOBAL__`, `ALERTING__`, `SCRAPE_CONFIGS__`, `REMOTE_WRITE__`, and `REMOTE_READ__` are used to configure Prometheus.

The formatting rules for these variable are as follows:

1. Environment keys will be transformed to lowercase.
2. Double underscore is used to go one level deeper in a yaml dictionary.
3. A single underscore followed by a number indicates the position of an array.

### Examples

The following are examples of using environmental variables to configure Prometheus. Pleaes consult the Prometheus configuration [documentation](https://prometheus.io/docs/prometheus/latest/configuration/configuration) for all configuration options.

- `GLOBAL__SCRAPE_INTERVAL=10s`

```yaml
global:
  scrape_interval: 10s
```

- `GLOBAL__EXTERNAL_LABELS=cluster=swarm`

```yaml
global:
  external_labels:
    cluster: swarm
    type: production
```

This is *NOT* `GLOBAL__EXTERNAL_LABELS__CLUSTER=swarm` because `CLUSTER` is not a standard Prometheus configuration. The `external_labels` option is a list of key values as shown in their documentation:

```yaml
external_labels:
  [ <labelname>: <labelvalue> ... ]
```

- `REMOTE_WRITE_1__URL=http://first.acme.com/write`, `REMOTE_WRITE_1__REMOTE_TIMEOUT=10s`,
`REMOTE_WRITE_2__URL=http://second.acme.com/write`

```yaml
remote_write:
- url: http://acme.com/write
  remote_timeout: 30s
- url: http://second.acme.com/write
```

Trailing numbers in the `REMOTE_WRITE_1` and `REMOTE_WRITE_2` prefixes dictates the position of the array of dictionaries.

- `REMOTE_WRITE_1__WRITE_RELABEL_CONFIGS_1__SOURCE_LABELS_1=label1`

```yaml
remote_write:
- write_relabel_configs:
  - source_labels: [label1]
```

## Scrape Environment Configuration

It is possible to add servers that are not part of the Docker Swarm Cluster just adding the variables `SCRAPE_PORT` and `SERVICE_NAME` on the environment. The project is going to use the [static_configs](https://prometheus.io/docs/operating/configuration/#<static_config>) configuration.

```
SCRAPE_PORT_1=1234
SERVICE_NAME_1=myservice.acme.com
SCRAPE_PORT_2=1234
SERVICE_NAME_2=myservice2.acme.com
```

You can also add a service via `api` using the `reconfigure` entry point.

```bash
curl `[IP_OF_ONE_OF_SWARM_NODES]:8080/v1/docker-flow-monitor/reconfigure?scrapePort=[PORT]&serviceName=[IP_OR_DOMAIN]&scrapeType=static_configs
```

Please consult [Prometheus Configuration](https://prometheus.io/docs/operating/configuration/) for more information about the available options.

## Scrape Secret Configuration

Additional scrapes can be added through files prefixed with `scrape_`. By default, all such files located in `/run/secrets` are automatically added to the `scrape_configs` section of the configuration. The directory can be changed by setting a different value to the environment variable `CONFIGS_DIR`.

The simplest way to add scrape configs is to use Docker [secrets](https://docs.docker.com/engine/swarm/secrets/) or [configs](https://docs.docker.com/engine/swarm/configs/).


## Scrape Label Configuration With Service and Node Labels

When using a version of [Docker Flow Swarm Listener](https://github.com/vfarcic/docker-flow-swarm-listener), DFSL, newer than `18.03.20-39`, you can configure DFSL to send node information to `Docker Flow Monitor`, DFM. This can be done by setting `DF_INCLUDE_NODE_IP_INFO` to `true` in the DFSL environment. DFM will automatically display the node hostnames as a label for each prometheus target. The `DF_SCRAPE_TARGET_LABELS` env variable allows for additional labels to be displayed.

In addition to service labels, DFM can be configured to import node and engine labels prefixed with `com.df.` as prometheus labels for our targets. First, configure DFSL to push node events to DFM by setting the following environment variables in DFSL:

1. `DF_NOTIFY_CREATE_NODE_URL=[MONITOR_DNS]:[MONITOR_PORT]/v1/docker-flow-monitor/node/reconfigure`
2. `DF_NOTIFY_REMOVE_NODE_URL=[MONITOR_DNS]:[MONITOR_PORT]/v1/docker-flow-monitor/node/remove`

Next set the following environment variables in DFM:

1. `DF_GET_NODES_URL=[SWARM_LISTENER_DNS]:[SWARM_LISTENER_PORT]/v1/docker-flow-swarm-listener/get-nodes`
2. `DF_NODE_TARGET_LABELS` to a comma seperate list of labels use.

!!! info
    Only `[a-zA-Z0-9_]` are valid characters in prometheus labels in `DF_NODE_TARGET_LABELS` and `DF_SCRAPE_TARGET_LABELS`.

For example, the stack file, [stacks/docker-flow-monitor-flexible-labels.yml](https://github.com/vfarcic/docker-flow-monitor/blob/master/stacks/docker-flow-monitor-flexible-labels.yml), configures DFM and DFSL to use service and node labels as prometheus labels. The `swarm-listener` service is defined as follows:

```yaml
services:
...
  swarm-listener:
    image: vfarcic/docker-flow-swarm-listener
    environment:
      - DF_NOTIFY_CREATE_SERVICE_URL=http://monitor:8080/v1/docker-flow-monitor/reconfigure
      - DF_NOTIFY_REMOVE_SERVICE_URL=http://monitor:8080/v1/docker-flow-monitor/remove
      - DF_NOTIFY_CREATE_NODE_URL=http://monitor:8080/v1/docker-flow-monitor/node/reconfigure
      - DF_NOTIFY_REMOVE_NODE_URL=http://monitor:8080/v1/docker-flow-monitor/node/remove
      - DF_INCLUDE_NODE_IP_INFO=true
...
```

The `DF_NOTIFY_*` environment variables defined in `swarm-listener` configures it to send service and node notifications to the `monitor` service. Setting `DF_INCLUDE_NODE_IP_INFO` to `true` configures `swarm-listener` to send node information for service changes.

The `monitor` service is defined as follows:

```yaml
services:
  monitor:
    image: vfarcic/docker-flow-monitor:${TAG:-latest}
    environment:
      ...
      - DF_SCRAPE_TARGET_LABELS=env,metricType
      - DF_NODE_TARGET_LABELS=aws_region,role
      - DF_GET_NODES_URL=http://swarm-listener:8080/v1/docker-flow-swarm-listener/get-nodes
```

The environment variable `DF_GET_NODES_URL` configures `monitor` to query `swarm-listener` for all nodes during startup. `DF_SCRAPE_TARGET_LABELS=env,metricType` configures `monitor` to use service labels `com.df.env` and `com.df.metricType` as prometheus labels with the `com.df.` prefix removed: `env` and `metricType` respectively. `DF_NODE_TARGET_LABELS=aws_region,role` configures `monitor` to use node label `com.df.aws_region` as a prometheus label with `com.df.` prefix remove: `aws_region`. The `role` target label is used by DFSL to denote the role of the node: `manager` and `worker`. For a complete list of node labels used by DFSL, head over to the [Docker Flow Swarm Listener Usage Docs](http://swarmlistener.dockerflow.com/usage/#node-notification).

For more information, please visit the [Flexible Labeling Tutorial](tutorial-flexible-labeling.md) to learn more about this feature!
