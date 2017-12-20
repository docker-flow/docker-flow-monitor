# Migration Guide

*Docker Flow Monitor* (DFM) now supports Prometheus 2! Prometheus 2 includes a new storage subsystem that has reduce CPU usage and lower disk space usage compared to Prometheus 1.  This guide highlights issues that you may encounter when upgrading to Prometheus 2.

## Database

When upgrading from DFM backed by Prometheus 1 to Prometheus 2, DFM will create a new database supported by Prometheus 2. If you need to access the previous data scraped by Prometheus 1, downgrade your DFM instance using tag: `17.12.12-36`. DFM will launch with Prometheus 1, and continue using the previous database.

## Backwards Compatibility

The command line arguments for Prometheus 2 has changed. This is detailed in Prometheus's [Official Migration Guide](https://prometheus.io/docs/prometheus/latest/migration/). *DFM* will continue to support the previous command line arguments prefixed by `ARG_`:

1. Setting `ARG_ALERTMANAGER_URL` configures the alertmanager correctly for Prometheus 2.
2. `ARG_STORAGE_LOCAL_PATH` maps to `--storage.tsdb.path`.
3. `ARG_STORAGE_LOCAL_RETENTION` maps to `--storage.tsdb.retention`.
4. `ARG_QUERY_STALENESS-DELTA` maps to `--query.lookback-delta`.
5. Setting `ARG_ENABLE-REMOTE-SHUTDOWN=true` sets flag `--web.enable-lifecycle`

You can explore the new flags in Prometheus 2 by downloading the binary from their [Download Page](https://prometheus.io/download/) and running `./prometheus -h`.
