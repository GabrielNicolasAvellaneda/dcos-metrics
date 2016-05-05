# Slave Module

Monitoring component to be run against ```mesos-slave```s. Contains an Isolator Module which tracks Task bringup/shutdown and advertises StatsD endpoints into Task environments. This module is included in EE versions of DCOS starting with 1.7, see the [dcos-image package](https://github.com/mesosphere/dcos-image/blob/master/packages/mesos-metrics-module/).

## Prerequisites

- CMake
- [Mesos' build prerequisites](http://mesos.apache.org/gettingstarted/)
- Boost ASIO (install ```libasio-dev```)

## Build instructions

Building this module requires a local copy of the mesos source, as well as a build from that source. You can use the provided ```get-mesos.sh``` script to get those.

```
host:dcos-stats$ ... install mesos prereqs ...
host:dcos-stats$ sh get-mesos.sh 0.26.0 # or whatever version you need
```

Once mesos is built, you can build the module code.

```
host:dcos-stats/slave$ sudo apt-get install \
  build-essential cmake libasio-dev libboost-system-dev libgoogle-glog-dev
host:dcos-stats/slave$ mkdir -p build; cd build
host:dcos-stats/slave/build$ cmake -Dmesos_VERSION=0.26.0 .. # match version passed to get-mesos.sh
host:dcos-stats/slave/build$ make -j4
host:dcos-stats/slave/build$ make test
```

If you already have a build of mesos available elsewhere, you can just point the stats module to that. For example, here's how to build on a DCOS node, which already has most of what we need within ```/opt/mesosphere```, except for ```libboost_system``` which isn't yet included as of this writing:

```
host:dcos-stats/slave$ sudo yum install cmake boost-system
host:dcos-stats/slave$ mkdir -p build; cd build
host:dcos-stats/slave/build$ cmake \
  -Dmesos_INCLUDE_DIR=/opt/mesosphere/include \
  -Dmesos_LIBRARY=/opt/mesosphere/lib/libmesos.so \
  -Dboost_system_LIBRARY=/usr/lib64/libboost_system.so.1.53.0 \
  -DUSE_LOCAL_PICOJSON=false \
  -DUSE_LOCAL_PROTOBUF=false \
  -DTESTS_ENABLED=false \
  .. # tests off to avoid CMake bug on some OSes
host:dcos-stats/slave/build$ make -j4
```

## Installing a custom build of the module

This is mainly for reference if you're doing module development. Otherwise you can just use the preconfigured version that's included on DCOS EE 1.7+.

The metrics module must be installed on **EACH** mesos-slave system that you want to forward metrics from.
It's recommended that you try these steps end-to-end on a single mesos-slave before continuing to other mesos-slaves, to ensure that you have the configuration you want BEFORE deploying it across the cluster.

1. Build the module against a version of Mesos matches what your cluster is running.
2. Copy the customized `libstats-slave.so` (and any additional dependency libs) into `/opt/mesosphere/lib/`
3. Back up the current versions of `/opt/mesosphere/etc/mesos-slave-common` and `/opt/mesosphere/etc/mesos-slave-modules.json`.
4. Perform the following changes to `mesos-slave-common` and `mesos-slave-modules.json` as needed:
  - `mesos-slave-common`
    - Append `,com_mesosphere_StatsIsolatorModule` to the end of the line defining `MESOS_ISOLATION`.
    - Add a line defining `MESOS_RESOURCE_ESTIMATOR=com_mesosphere_StatsResourceEstimatorModule`.
    - REMOVE any existing line defining `MESOS_HOOKS=com_mesosphere_StatsEnvHook`. Recent builds of the module no longer need this.
  - `mesos-slave-modules.json`
    - Add a configuration block for `/opt/mesosphere/lib/libstats-slave.so` which lists `com_mesosphere_StatsIsolatorModule` and `com_mesosphere_StatsResourceEstimatorModule`. The latter replaces the previously needed `com_mesosphere_StatsEnvHook`.
5. Make any other changes to settings in `mesos-slave-modules.json` as needed. See below.
6. Last chance to back out! Revert `mesos-slave-common` and `mesos-slave-modules.json` to their original state if you want to abort now. The library files added to `/opt/mesosphere/lib/` are effectively unused until the configs in `/opt/mesosphere/etc/` are referencing them.
7. Restart the `mesos-slave` process, see below.
8. Verify that the module is working by checking `mesos-slave` logs, see below.

## Configuring/customizing the module

All configuration is within `/opt/mesosphere/etc/mesos-slave-modules.json`. The `mesos-slave` process must be restarted for any changes to take effect (see below for how to do this). Here are some explanations of the parameters:
- **"`dest_host`": Hostname/IP for where to forward data received from tasks.**
- "`dest_refresh_seconds`": Duration in seconds between DNS lookups of dest_host. Automatically detects changes in the DNS record and redirects output to the new destination.
- "`dest_port`": Port to use when forwarding stats to dest_host.
- "`annotation_mode`": How (or whether) to tag outgoing data with information about the Mesos task. Available modes are "key_prefix" (prefix statsd keys with the info), "tag_datadog" (use the datadog tag extension), or "none" (no tagging, data forwarded without modification). **If your statsd receiver doesn't support datadog-format statsd tags, this should be 'key_prefix' or 'none'.**
- "`chunking`": Whether to group outgoing data into a smaller number of packets. **If your statsd receiver doesn't support multiple newline-separated statsd records in the same UDP packet, this should be 'false'.**
- "`chunk_size_bytes`": Preferred chunk size for outgoing UDP packets, when "chunking" is enabled. This should be the UDP MTU.

The full list of config options is in [params.hpp](params.hpp).

IMPORTANT: Again, any changes to these options don't take effect until the mesos-slave process is restarted using the following steps:

## Restarting `mesos-slave`

The `mesos-slave` process must be restarted for any module config changes to take effect. This may or may not require dropping the containers managed by that process.

1. Copy the current slave state into a backup location:
  - `$ cp -a /var/lib/mesos/slave /var/lib/mesos/slave.bak`
2. Restart the `mesos-slave` process:
  - `$ systemctl restart dcos-mesos-slave` (or `dcos-mesos-slave-public`)
3. Check `mesos-slave`'s status (any of these):
  - `$ systemctl status dcos-mesos-slave` (or `dcos-mesos-slave-public`)
  - `$ journalctl -f`
  - `$ journalctl -e -n 100`

## Verifying the module works (any of the following)

Check the mesos-slave logs for something like this near the beginning:

```input_assigner_factory.cpp:31 Creating new stats InputAssigner with parameters: [... json config content …]```

Check the mesos-slave logs for a message like this every minute (assuming a 'metrics' process hasn't been started yet, as described below):

```port_writer.cpp:180 Error when resolving host[metrics.marathon.mesos]. Dropping data and trying again in 60 seconds. err=asio.netdb:1, err2=system:22```

Start a process in marathon that just runs `env` and look for envvars named `STATSD_UDP_HOST` and `STATSD_UDP_PORT`, or run the `test-sender` process as described in [DEMO.md](../DEMO.md).

## Uninstalling the module

1. Undo the changes made to `/opt/mesosphere/etc/mesos-slave-common` and `/opt/mesosphere/etc/mesos-slave-modules.json` by restoring the original files (you kept backups, right?).
2. The library files added to `/opt/mesosphere/lib/` should also be backed out.
3. Restart `mesos-slave` using the steps described before.
