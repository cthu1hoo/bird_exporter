# bird_exporter

Metric exporter for bird routing daemon to use with Prometheus.

## Remarks

Since bird_exporter uses the bird unix sockets, bird has to be installed on the same machine as bird_exporter. Also the user executing bird_exporter must have permission to access the bird socket files.

### Bird configuration

To get meaningful uptime information bird has to be configured this way:

```
timeformat protocol     iso long;
```

## Important information for users of bird 2.0+

Version 2.0 of bird routing daemon does support IPv4 and IPv6 in one single daemon now.
For further information see [here](https://gitlab.labs.nic.cz/labs/bird/wikis/transition-notes-to-bird-2).
Since version 1.1 bird_exporter can be used with bird 2.0+ using the `-bird.v2` parameter.
When using this parameter bird_exporter queries the same bird socket for IPv4 and IPv6.
In this mode the IP protocol is determined by the channel information and parameters `-bird.ipv4`, `-bird.ipv6` and `-bird.socket6` are ignored.

Bird 3.0 adds two extra columns (RX limit, limit) to route change stats. The exporter detects this format automatically.

## Metric formats

In version 1.0 a new metric format was introduced.
To prevent a breaking change the new format is optional and can be enabled by using the `-format.new` flag.
The new format handles protocols more generic and allows a better query structure.
Also it adheres more to the metric naming best practices.
In both formats protocol specific metrics are prefixed with the protocol name (e.g. OSPF running metric).

Since verson 1.3 the new metric format is the default.

This is a short example of the different formats:

### old format

```
bgp4_session_prefix_count_import{name="bgp1"} 600000
bgp6_session_prefix_count_import{name="bgp1"} 50000
ospfv3_running{name="ospf1"} 1
```

### new format

```
bird_protocol_prefix_import_count{name="bgp1",proto="BGP",ip_version="4"} 600000
bird_protocol_prefix_import_count{name="bgp1",proto="BGP",ip_version="6"} 50000
bird_ospfv3_running{name="ospf1"} 1
```

### Default Port

In version 0.7.1 the default port changed to 9324 since port 9200 is the default port of Elasticsearch. The new port is now registered in the default port allocation list (https://github.com/prometheus/prometheus/wiki/Default-port-allocations)

### Sockets

In version 0.8 communication to bird changed to sockets. The default socket path is `/var/run/bird.ctl` (for bird) and `/var/run/bird6.ctl` (for bird6). In case you are using different paths in your installation, the socket path can be specified by usind the `-bird.socket` (for bird) and `-bird.socket6` (for bird6) flag.

The BIRD control socket is a privileged command channel. Prefer a dedicated
restricted BIRD CLI socket where the BIRD version supports it, and grant the
exporter process access only to that socket.

The standalone binary listens on all interfaces for compatibility. Bind to
loopback, apply a network allowlist, or put the endpoint behind an
authenticating TLS reverse proxy when the metrics network is not trusted.

## Install

```
go get -u github.com/czerwonk/bird_exporter
```

or

```
go install github.com/czerwonk/bird_exporter@latest
```

## Kubernetes

To run the bird_exporter in Kubernetes, you can use the supplied helm chart

### Prerequisites

- Helm v3.0.0+

### Installing the chart

To install the chart with the release name `bird-exporter`:

```console
$ helm repo add bird-exporter "https://czerwonk.github.io/bird_exporter"
"bird-exporter" has been added to your repositories

$ helm repo update
Hang tight while we grab the latest from your chart repositories...
...Successfully got an update from the "bird-exporter" chart repository
Update Complete. ⎈Happy Helming!⎈

$ helm install bird-exporter bird-exporter/bird-exporter
NAME: bird-exporter
...

```

The chart uses `ghcr.io/czerwonk/bird_exporter` and runs as UID/GID 1000 with
all capabilities dropped, a read-only root filesystem, the RuntimeDefault
seccomp profile, and no Kubernetes service-account token. Set `fsGroup` to the
numeric group that can access the selected BIRD socket. A read-only mount does
not make the BIRD command protocol read-only.

The container image also runs as UID/GID 1000 outside Kubernetes. Add only the
host socket group with Docker's `--group-add` option instead of running the
container as root. Set `TZ` to the BIRD host timezone when BIRD emits local
timestamps without a timezone.

## Usage

```
bird_exporter -format.new=true
```

## BIRD Grafana Dashboard

There are three Grafana dashboards available for this exporter. Check the [grafana](https://github.com/czerwonk/bird_exporter/tree/main/grafana) for more.

## Features

- BGP session state
- OSPF neighbor/interface count
- OSPF topology: adjacencies per interface, LSA database, SPF results and routes (opt-in, see below)
- imported / exported / filtered prefix counts / route state changes (BGP, OSPF, Kernel, Static, Device, Direct, Babel)
- protocol uptimes (BGP, OSPF, BFD)
- BFD session status

## OSPF topology and protocol status metrics

The OSPF exporter answers "which site is adjacent to which site, over which
interface, at what cost, and is the adjacency actually up?". The metrics are
opt-in, because they add series per interface, per LSA and per route:

```
bird_exporter -ospf.interface -ospf.neighbors -ospf.lsdb -ospf.topology -ospf.routes
```

Every interface of the protocol is reported, even when no neighbor answers, so
a broken link reads as `bird_ospf_link_up 0` instead of disappearing.

| Metric | Meaning |
|--------|---------|
| `bird_ospf_link_up{name,area,local_router_id,local_iface,local_network}` | 1 only when the interface has a neighbor in state Full (or Establishing for NBMA) |
| `bird_ospf_link_info{...,remote_router_id,remote_ip,state,role,priority}` | the far end of every adjacency that exists |
| `bird_ospf_neighbor_state{...,neighbor_id}` | OSPF finite state machine: Init=2, 2Way=3, ExStart=4, Exchange=5, Loading=6, Full=7 |
| `bird_ospf_neighbor_hold_timer_seconds{...,neighbor_id}` | remaining hold timer, sawtooth 20 -> 0 with `dead 20` |
| `bird_ospf_interface_info{...,iface,area,network,type,state,stub}` | every configured OSPF interface, `stub=yes` for passive ones |
| `bird_ospf_interface_cost{...,iface}` | configured interface cost |
| `bird_ospf_interface_dr_info{...,dr_id,dr_ip,bdr_id,bdr_ip}` | designated router of a broadcast segment |
| `bird_ospf_lsa_count{name,area,type}` | LSA count per type and area (`area` is empty for the global table) |
| `bird_ospf_lsa_age_seconds` / `bird_ospf_lsa_sequence` | per LSA, only with `-ospf.lsdb.details` |
| `bird_ospf_spf_distance{name,area,root,root_kind}` | root of each SPF run, distance 0 = this router |
| `bird_ospf_spf_node_metric{...,kind,peer,prefix}` | cost from the root to a node, cheapest path wins |
| `bird_ospf_spf_network_router{...,network,dr,member}` | members of a transit segment, `is_dr` labels the DR |
| `bird_ospf_route_info{...,prefix,advertised_by,nexthop_ip,iface,type}` | OSPF routes and who advertises them |
| `bird_ospf_route_metric1` / `bird_ospf_route_best` | metric and best-route flag per route |

`local_router_id` carries the router ID of the local instance, so a map can join
the metrics of many routers by router ID without extra configuration. The
router ID and the configured hostname are also published as labels of
`bird_daemon_info`.

Note that the topology metrics print the area as a dotted quad (`area="0.0.0.0"`),
while the older `bird_ospf_*_count` metrics use the short form (`area="0"`).

## Third Party Components

This software uses components of the following projects

- Prometheus Go client library (https://github.com/prometheus/client_golang)

## License

(c) Daniel Czerwonk, 2016. Licensed under [MIT](LICENSE) license.

## Prometheus

see https://prometheus.io/

## Bird routing daemon

see http://bird.network.cz/
