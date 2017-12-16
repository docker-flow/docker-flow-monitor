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
prometheus web.route-prefix=/monitor web.external-url=http://localhost/monitor
```

`ARG` variables defined by default are as follows.

```
ARG_CONFIG_FILE=/etc/prometheus/prometheus.yml
ARG_STORAGE_LOCAL_PATH=/prometheus
ARG_WEB_CONSOLE_LIBRARIES=/usr/share/prometheus/console_libraries
ARG_WEB_CONSOLE_TEMPLATES=/usr/share/prometheus/consoles
```

## Global Configuration

Environment variables prefixed with `GLOBAL_` are used instead Prometheus global entries in the configuration.

The formatting rules for the `GLOBAL` variables are as follows:

1. Variable name has to be prefixed with `GLOBAL_`.
2. Capital letters will be transformed to lower case.

For example, if environment variable `GLOBAL_SCRAPE_INTERVAL=10s` is defined, the resulting Prometheus configuration would be as follows.

```
global:
  scrape_interval: 10s
```

Nested values can be specified by separating them with `-`. For example, if environment variables `GLOBAL_EXTERNAL_LABELS-CLUSTER=swarm` and `GLOBAL_EXTERNAL_LABELS-TYPE=production` are defined, the resulting Prometheus configuration would be as follows.

```
global:
  external_labels:
    cluster: swarm
    type: production
```

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

## Remote Read Configuration

Environment variables prefixed with `REMOTE_READ_` are used instead Prometheus `remote_read` entries in the configuration.

The formatting rules for the `REMOTE_READ` variables follow the same pattern as those used for [Global Configuration](#global-configuration)

## Remote Write Configuration

Environment variables prefixed with `REMOTE_WRITE_` are used instead Prometheus `remote_write` entries in the configuration.

The formatting rules for the `REMOTE_WRITE` variables follow the same pattern as those used for [Global Configuration](#global-configuration)

## Scrapes

Additional scrapes can be added through files prefixed with `scrape_`. By default, all such files located in `/run/secrets` are automatically added to the `scrape_configs` section of the configuration. The directory can be changed by setting a different value to the environment variable `CONFIGS_DIR`.

The simplest way to add scrape configs is to use Docker [secrets](https://docs.docker.com/engine/swarm/secrets/) or [configs](https://docs.docker.com/engine/swarm/configs/).
