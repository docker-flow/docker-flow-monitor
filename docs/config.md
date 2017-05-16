# Configuring Docker Flow Monitor

*Docker Flow Monitor* can be configured through Docker environment variables and/or by creating a new image based on `vfarcic/docker-flow-monitor`.

## Environment Variables

!!! tip
	The *Docker Flow Monitor* container can be configured through environment variables

The environment variables used to configure *Docker Flow Monitor* are divided into three groups distinguished by variable prefixes.

### ARG variables

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

### GLOBAL variables

Environment variables prefixed with `GLOBAL_` are used instead Prometheus global entries in the configuration.

The formatting rules for the `GLOBAL` variables are as follows:

1. Variable name has to be prefixed with `GLOBAL_`.
2. Capital letters will be transformed to lower case.

For example, if environment variable `GLOBAL_SCRAPE_INTERVAL=10s` is defined, the resulting Prometheus configuration would be as follows.

```
global:
  scrape_interval: 10s
```

Please consult [Prometheus configuration](https://prometheus.io/docs/operating/configuration/) for more information about the available options.