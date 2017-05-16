# Configuring Docker Flow Monitor

*Docker Flow Monitor* can be configured through Docker environment variables and/or by creating a new image based on `vfarcic/docker-flow-monitor`.

## Environment Variables

!!! tip
	The *Docker Flow Monitor* container can be configured through environment variables

The environment variables used to configure *Docker Flow Monitor* are divided into three groups distinguished by variable prefixes.

### ARG variables

Environment variables prefixed will `ARG_` are used instead Prometheus startup arguments.

### GLOBAL variables

Environment variables prefixed with `GLOBAL_` are used instead Prometheus global entries in the configuration.

For example, if environment variable `GLOBAL_SCRAPE_INTERVAL=10s` is defined, the resulting Prometheus configuration would be as follows.

```
global:
  scrape_interval: 10s
```

Please consult [Prometheus configuration](https://prometheus.io/docs/operating/configuration/) for more information about the available options.