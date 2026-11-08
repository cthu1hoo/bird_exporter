# OSPF topology and protocol status metrics — feasibility research

Goal: enough data in Prometheus to build a site/routing map in a separate project —
which site links to which, over which interface, with which IPv4 addresses, at what cost,
and whether the adjacency is up.

Scope of this note: what BIRD actually exposes, what we scrape today, what the new metrics
would look like, and how the code would change. Everything marked *verified* was run against
BIRD 2.17.5 (Debian trixie `bird2` package) in this sandbox.

## 1. What we scrape today

| Source | Code | OSPF data |
|---|---|---|
| `show protocols all` | `parser/parser.go`, `metrics/generic_exporter.go` | `bird_protocol_up{state=...}`, uptime, prefix import/export/filter counts, route-change counters |
| `show ospf <name>` | `parser/ospf.go`, `metrics/ospf_exporter.go` | `bird_ospf_running`, and per area: `bird_ospf_interface_count`, `bird_ospf_neighbor_count`, `bird_ospf_neighbor_adjacent_count` |
| `show status` | `parser/status.go`, `metrics/status_exporter.go` | `bird_daemon_up`, `bird_daemon_info{router_id,version}`, last reboot/reconfig, `bird_socket_query_success` |

Conclusion: OSPF coverage is four counters plus a derived up/down flag. No neighbor identity,
no interface identity, no addresses, no costs, no prefix reachability. A map cannot be built
from this. The assessment "current metrics are insufficient" is correct.

Note: `show protocols all` already reports the OSPF finite state machine in the `state` label
(`Alone` = up with no adjacency, verified). That is a useful health signal we already have.

## 2. BIRD CLI surface for OSPF (verified on 2.17.5)

The instance name goes **after** the subcommand. `show ospf ospf1 state` is a syntax error;
`show ospf state ospf1` is correct. The LSDB command is `lsadb`, not `lsdb`.

| Command | Verified output | Gives |
|---|---|---|
| `show ospf [name]` | area blocks with `Number of interfaces / neighbors / adjacent neighbors`, plus `Number of LSAs in DB`, `Stub router`, `Stub/NSSA/Transit` per area | counts (today's data) + LSDB size and area flags, which we do **not** parse today |
| `show ospf neighbors [name]` | table: `Router ID, Pri, State, DTime, Interface, Router IP` | **the link edge**: local interface name, neighbor interface IPv4, neighbor router ID, adjacency state, priority |
| `show ospf interface [name]` | per interface block: `Interface ifA1 (10.1.0.0/24)`, `Type: ptp`, `Area:`, `State: PtP`, `Priority:`, `Cost:`, `ECMP weight:`, timers, and for broadcast/NBMA `Designated router (ID)/(IP)`, `Backup designated router (ID)/(IP)` | local interface → IPv4 network, area, cost, state |
| `show ospf lsadb [name]` | `Global` and per-`Area` tables: `Type, LS ID, Router, Sequence, Age, Checksum` | full LSA inventory: one `0001` per router in the area, `0002` networks, `0003` inter-area, `0005` externals; churn/aging health |
| `show ospf topology [all] [name]` | one block per router: `router <id>`, `distance <n>`, then `router <peer> metric <n>` / `network <net> metric <n>` | **computed SPF result, not adjacency**: cost from that root to every other router/network. `topology` and `topology all` produced identical output in the live capture |
| `show ospf state [all] [name]` | same blocks plus `stubnet`, `external` (and `xnetwork`/`xrouter` in multi-area) | per-router prefix reachability with cost. `state` and `state all` were identical in the live capture |

Verified single-router samples (BIRD 2.17.5):

```
$ birdcl "show ospf interface"
ospf1:
Interface ifA1 (10.1.0.0/24)
	Type: ptp
	Area: 0.0.0.0 (0)
	State: PtP
	Priority: 1
	Cost: 10
	ECMP weight: 1
	Hello timer: 2
	Wait timer: 8
	Dead timer: 8
	Retransmit timer: 5

$ birdcl "show ospf lsadb"

Area 0.0.0.0

 Type   LS ID           Router          Sequence   Age  Checksum
 0001  1.1.1.1         1.1.1.1          80000001    19    c330

Global

 Type   LS ID           Router          Sequence   Age  Checksum
 0005  10.10.1.0       1.1.1.1          80000001    19    bfc2

$ birdcl "show ospf state"

area 0.0.0.0

	router 1.1.1.1
		distance 0
		stubnet 10.1.0.0/24 metric 10
		external 10.10.1.0/24 metric2 10000
```

Multi-router line shapes (from the printf formats inside the binary; the same formats exist in
BIRD 3.1.7):

```
		distance %u
		vlink %R metric %u
		router %R metric %u              <- point-to-point / transit edge to another router
		network %I/%d metric %u          <- transit network node
		network [%R] metric %u          <- network LSA keyed by DR router id
		network [%R-%u] metric %u
		stubnet %I/%d metric %u          <- router's own stub networks
		xnetwork %N metric %u            <- inter-area (type 3)
		xrouter %R metric %u
		%s %N metric%s %u%s%s            <- external (type 5/7), metric1/metric2, via
```

`show ospf neighbors ospf1` header verified; the row format is one neighbor per line with the
six columns above.

Correction after the live capture (see section 8): the `router <peer> metric <n>` lines under a
root are **SPF results** (cost to reach that peer from that root), not link adjacencies. The
adjacency/link table comes from `show ospf neighbors` joined with `show ospf interface`.
`topology`/`state` are still valuable: they give the directed cost matrix for all router pairs
and per-router prefix reachability, which is what a map uses to draw distances and to detect
asymmetric link costs.

Key structural point for the map project: **one** BIRD instance holds the LSDB of the whole
area. So a single site's exporter already sees every router in area 0 (`0001` LSAs) and the
full SPF tree (`topology all`). Per-site exporters then give independent vantage points of the
same graph, which is exactly what is needed to spot one-way links, area mismatches, and
split-site adjacencies.

## 3. Proposed metrics

Naming and label style follow the existing `bird_<proto>_*` convention. Labels `name`
(protocol instance, `ospf1` for the production config) and `area` are kept on every metric.

Group A — edges, **interface first** (cheap, enable by default)

Live observation (section 8.4): a peer that stops answering disappears from
`show ospf neighbors`. `show ospf interface` still lists the interface. So every configured link
must be emitted from the interface table, and the neighbor identity attached only when present.
A link metric derived from the neighbor table alone would make dead links disappear from the map
instead of showing them as down.

```
bird_ospf_interface_info{name,area,iface,network,type,state,stub}                   1
bird_ospf_interface_cost{name,area,iface}                                          gauge
bird_ospf_link_up{name,area,local_router_id,local_iface,local_network}              1 = adjacency Full, 0 otherwise
bird_ospf_link_info{name,area,local_router_id,local_iface,local_network,
                    remote_router_id,remote_ip,state,role,priority}                1  (absent = no adjacency)
bird_ospf_neighbor_state{name,area,iface,neighbor_id,neighbor_ip}                   gauge, FSM index (0 Down .. 7 Full)
bird_ospf_interface_dr_info{name,area,iface,dr_id,dr_ip}                            1 (broadcast/NBMA only)
bird_ospf_area_info{name,area,stub,nssa,transit,backbone}                           1 (from show ospf)
```

`local_router_id` and `local_network` come from `show status` and `show ospf interface`, so each
series is self-describing and needs no join. `bird_ospf_link_info` is the edge table: dedupe on
the unordered router-ID pair in the mapping project, use `remote_ip` as the WireGuard endpoint.
The map query for "links that exist but are not adjacent" is simply interfaces without a matching
`bird_ospf_link_info` row (`wg_server`, `br_float` in the live capture).

`DTime` from the neighbor table is left out of the default set: it behaves like the remaining
hold time, not an age (section 8.4). If exported at all it must be named
`bird_ospf_neighbor_hold_timer_seconds` with that meaning documented.

Group B — prefix and route view (opt-in, `-ospf.routes`)

```
bird_ospf_route_info{name,prefix,advertised_by,nexthop_ip,iface,route_type}         1
bird_ospf_route_metric{name,prefix,metric1}
```

From `show route all protocol ospf1` (note the `all` keyword; `detail` is a syntax error there).
It carries `OSPF.router_id`, `OSPF.metric1`, and `via <ip> on <iface>` / `dev <iface>`, i.e.
prefix to site to interface in one table. 30 routes at this site, so it is cheap, but it grows
with the routing table and stays opt-in.

Group C — SPF tree (opt-in flag, medium cardinality)

```
bird_ospf_spf_distance{name,area,router}                                           cost to router
bird_ospf_spf_edge{name,area,router,edge_type=router|network|stubnet|vlink,
                    peer,peer_ip,prefix}                                           1
bird_ospf_spf_prefix{name,area,router,prefix,kind,metric}                          1
```

Group D — LSDB health (opt-in flag, cardinality grows with LSA count)

```
bird_ospf_lsas_in_db{name}                                    currently parsed but not exported
bird_ospf_lsa_count{name,area_or_global,type}                 type = 0001|0002|0003|0005
bird_ospf_router_count{name}                                  count of 0001 LSAs = routers in area
bird_ospf_lsa_age_seconds{name,area,type,ls_id,router}
bird_ospf_lsa_sequence{name,area,type,ls_id,router}           rising counter deltas = LSA churn
```

Measured at one production site (section 8.3): groups A-D together are about 300-400 series.
Group C and D scale with the SPF node count and the LSDB size, so they stay behind flags for
larger deployments even though they can be on by default at this size.

## 4. Implementation plan (fits the current layout)

1. `protocol/` — new structs: `OSPFNeighbor{RouterID, Priority, State, Role, DTime, Interface,
   RouterIP}`, `OSPFInterface{Name, Network, Type, Area, State, Stub, Cost, ECMPWeight, Timers,
   DRID, DRIp}`, `OSPFLSA{Area, Type, LSID, Router, Sequence, Age, Checksum}`,
   `OSPFTree{Area, RootRouter, Distance, Nodes[]}`, `OSPFRoute{Prefix, RouterID, Metric1,
   NextHopIP, Interface, Type}`. Also add `Hostname` to `parser.Status` (live BIRD prints
   `Hostname is egw-spb1.fakecake.org`) and expose it as a label on `bird_daemon_info`.
2. `client/client.go` — extend the `Client` interface with `GetOSPFNeighbors`,
   `GetOSPFInterfaces`, `GetOSPFDatabase`, `GetOSPFTopology(p, all bool)`,
   `GetOSPFRoutes(p)`; implement in `client/bird_client.go` with
   `birdsocket.Query(sock, "show ospf neighbors " + p.Name)`,
   `show route all protocol <name>` etc.
   `bird_socket` already takes an arbitrary command string, so no dependency change.
3. `parser/` — `ospf_neighbors.go` (column table), `ospf_interface.go` (indented key/value
   blocks), `ospf_lsadb.go` (fixed-ish columns, `Global` vs `Area <id>` sections),
   `ospf_tree.go` (indent-based tree: `area`, `router`, `distance`, node lines),
   `ospf_routes.go` (route block: header line with `(pref/metric) [rid]`, indented `via`/`dev`
   and `OSPF.metric1` / `OSPF.router_id` attributes). Split every table row on any whitespace run,
   never on fixed widths: one-shot `birdc "cmd"` uses tabs, an interactive terminal pads with
   spaces (section 8.4). Reuse `parseInt`, `parseFloat`, and the existing scanner size limit
   (`maxProtocolLineBytes`) as in `parser/ospf.go`.
4. `metrics/ospf_topology_exporter.go` — a second OSPF exporter registered next to
   `NewOSPFExporter` in `exportersForDefault`/`exportersForLegacy` (`metric_collector.go`). It
   fetches interfaces and neighbors in one pass, keys them by interface name, and emits
   `bird_ospf_link_up` = 0 for interfaces with no neighbor row (the interface-first rule of
   section 3).
5. `main.go` — flags `-ospf.interface` (true), `-ospf.neighbors` (true), `-ospf.lsdb` (false),
   `-ospf.topology` (false), `-ospf.routes` (false).
6. Query cost: the collector is rebuilt per HTTP request and re-queries BIRD on every gather.
   Adding 4 queries per OSPF instance per scrape is acceptable for one Prometheus, but add a
   small TTL cache (5–10 s) in `BirdClient` if several scrapers hit the exporter or the scrape
   interval is short. BIRD's CLI socket serialises commands, so bursts increase BIRD latency.
7. Tests: table-driven parser tests with captured fixtures, in the style of
   `parser/ospf_test.go`, plus a fake `client.Client` so the new exporter can be tested without
   a socket (today `metrics/ospf_exporter_test.go` passes `nil`).
8. Docs/README: new metric list, flags, cardinality warning, and a note that the BIRD CLI
   socket is not a read-only channel (already in README).

Estimated size: ~400–600 lines of Go including tests. No new dependencies.

## 5. Open questions and risks

1. **Multi-router fixtures.** The adjacency-level output (populated `neighbors` rows,
   `router <id> metric <n>` tree lines, multiple `0001` LSAs) is derived from BIRD's own format
   strings, not captured, because OSPF adjacencies cannot form in this sandbox: the kernel here
   does not deliver IPv4 multicast to raw sockets. Verified: raw protocol-89 **unicast**
   delivery works, multicast does not, both in a private network namespace and across podman
   bridges. This is a sandbox property, not a BIRD or exporter issue. Live captures are needed
   before the parsers are written (see section 6).
2. **Router ID → site mapping.** The map project needs a name for each router ID. Options: a
   convention (router ID = a fixed loopback/anycast IPv4), or a label from
   `-format.description-labels` on the OSPF protocol instance (`protocol ospf v2 "ospf1" {
   description "site=ck"; }`), which the exporter already turns into labels.
3. **BIRD 2 vs BIRD 3.** Same CLI classes in 3.1.7 (`cmd_SHOW_OSPF_TOPOLOGY`,
   `cmd_SHOW_OSPF_STATE`, `cmd_SHOW_OSPF_LSADB`, `cmd_SHOW_OSPF_NEIGHBORS`,
   `cmd_SHOW_OSPF_INTERFACE`) and the same tree formats. Production uses `protocol ospf v2`, so
   OSPFv3 (ospfv3, IPv6 LSA ids, link-local next hops) is out of scope for now, but the parser
   must not assume IPv4-only columns if OSPFv3 is ever enabled.
4. **Neighbor state semantics.** BIRD neighbor states: Down, Attempt, Init, 2-Way, ExStart,
   Exchange, Loading, Full. Only `Full` is adjacent. `bird_ospf_neighbor_up` should follow
   "adjacent", while `state` stays a label for diagnosis.
5. **Multi-area / virtual links.** Production is single area 0. `vlink` lines and area
   translation would be needed for multi-area designs; keep the parser tolerant and log unknown
   node kinds instead of failing.
6. **Cardinality and PII-free output.** No secrets appear in these outputs (passwords are not
   shown by any `show ospf` command), but LSDB-derived metrics should be opt-in.

## 6. Data needed from a live instance

*Superseded by section 8: this list was fulfilled by `test-bird.txt` and `test-bird2.txt`; only
the items in 8.5 are still open.*

Run on one production router (BIRD 2.x, `protocol ospf v2` with no explicit instance name gets
the auto-name `ospf1`, verified):

```bash
bird -v | head -1
birdc "show protocols all"
birdc "show ospf"
birdc "show ospf neighbors"
birdc "show ospf interface"
birdc "show ospf lsadb"
birdc "show ospf state"
birdc "show ospf state all"
birdc "show ospf topology"
birdc "show ospf topology all"
birdc "show route protocol ospf1"
```

If the instance is named differently, pass it after the subcommand, e.g.
`birdc "show ospf neighbors myospf"`. Paste as text (no formatting loss matters for the column
parser). One capture from a router with at least two adjacencies and one capture from a
degraded one (an interface in `Waiting`/a neighbor not `Full`) would be ideal.

## 7. Reproducing the sandbox lab

The lab scripts live in `/tmp/birdlab/` in this session (not committed). Summary of what works:

```bash
apt-get download bird2 bird3          # Nexus Debian mirror
dpkg-deb -x bird2_*.deb x/            # no root needed
unshare -Urn bash lab.sh              # private netns, veth pairs, 2-3 bird instances
```

Adjacency does not form there (multicast, see 5.1), but every command's syntax and the
single-router output shape can be captured this way, which is how section 2 was produced.

## 8. Live capture review (`test-bird.txt`, host `egw-spb1`, BIRD 2.17.5, instance `ospf1`, area 0)

This capture is enough to design and unit-test the parsers. Contents: 5 neighbor rows,
7 interfaces, 30 LSAs (6 x `0001`, 1 x `0002`, 23 x `0005`), 30 OSPF routes, 6 SPF roots.
This section supersedes the request list in section 6.

### 8.1 Link table the map needs, derived from the capture

`show ospf neighbors` joined with `show ospf interface`:

| local iface | local net | neighbor router ID | neighbor IP | state/role |
|---|---|---|---|---|
| eth1 | 192.168.112.128/28 | 109.68.212.12 | 192.168.112.132 | Full/BDR |
| wg_home | 192.168.111.24/30 | 192.168.112.5 | 192.168.111.25 | Full/PtP |
| wg_egw_eu1 | 192.168.111.52/30 | 45.154.35.70 | 192.168.111.54 | Full/PtP |
| wg_egw_eu2 | 192.168.111.28/30 | 138.124.75.88 | 192.168.111.30 | Full/PtP |
| wg_ck | 192.168.111.8/30 | 192.168.112.6 | 192.168.111.10 | Full/PtP |
| wg_server | 192.168.111.192/28 | (no adjacency) | - | iface PtP, no neighbor |
| br_float | 192.168.112.3/32 | (no adjacency) | - | iface `Waiting (stub)` |

Local router ID `147.45.138.221` = `egw-spb1` `eth0`, confirmed by the user, and it is the SPF
root with `distance 0` and the DR of `192.168.112.128/28`. **Router ID equals the host's public
IPv4**, so the map gets site identity for free from every table; no side mapping required. A
consistency check is still worth exporting (`bird_daemon_info{router_id}` already exists).

The `network 192.168.112.128/28` node lists the routers attached to the multi-access segment
(`dr 147.45.138.221`, `router 147.45.138.221`, `router 109.68.212.12`), which cross-checks the
neighbor table. `192.168.112.5` and `192.168.112.6` advertise their LANs as `stubnet
172.17.17x.0/24` / `172.18.17x.0/24`, so LAN subnets can be attached to sites as well.

### 8.2 Parser details learned

- Neighbor table is tab separated with columns `Router ID, Pri, State, DTime, Interface,
  Router IP`. The State column carries the network role: `Full/PtP`, `Full/BDR` (expect
  `Full/DR`, `Full/DROther`). Split into `state` plus `role` labels.
- `Pri` is 128 on ptp interfaces and 1 on the broadcast interface.
- `DTime` prints as `18.398`; see 8.4 - it looks like the remaining hold timer, not an age.
- SPF blocks may list the same peer twice with different metrics (`router 192.168.112.6
  metric 10` and `metric 15` under `109.68.212.12`) - BIRD keeps several candidate paths. The
  parser must accept duplicates; the exporter should emit the minimum metric plus a path count.
- Costs are directed: `147.45.138.221 -> 192.168.112.6` is 10, `192.168.112.6 ->
  147.45.138.221` is 30 (different per-direction interface costs). The map must model directed
  edges with per-direction cost.
- `distance 0` marks the local root; all other roots show `distance 10`.
- Interfaces without adjacency appear only in `show ospf interface`, so dead or half-configured
  links (here `wg_server`, `br_float`) are detectable.
- 23 type-5 externals come from the redistribution filter in the production config; the
  advertising-router column makes `bird_ospf_lsa_count{type="0005",router=...}` a per-site
  "how many prefixes does this site inject" signal.

### 8.3 Measured cardinality

At this site: 5 neighbors, 7 interfaces, 30 LSAs, 30 routes, ~70 SPF nodes. All four metric
groups together are roughly 300-400 series per site. At this fleet size every group can be on by
default; keep the flags for larger deployments and for scraping with short intervals.

### 8.4 Second capture (`test-bird2.txt`, same host) and what it settled

- `show status` matches the existing parser: `BIRD 2.17.5`, `Router ID is 147.45.138.221`,
  `Current server time is 2026-11-08 22:38:53.463`, `Last reboot on ...`, `Daemon is up and
  running`. Verified against `parser.ParseStatusWithError` in a throwaway test: all three
  timestamps parse fine (Go accepts the fractional seconds although the layout omits them), so
  `bird_daemon_info`, `bird_last_reboot_timestamp_seconds` and friends work on this build.
- Unused extra line: `Hostname is egw-spb1.fakecake.org`. Worth adding to `Status` and to
  `bird_daemon_info` as a `hostname` label, which gives the map a site name per instance.
- Route command syntax is `show route all protocol ospf1`. The `detail` keyword is rejected in
  that position (`show route protocol ospf1 detail` is a syntax error; verified in the sandbox
  as well). `all` prints per-route attributes:
  ```
  192.168.112.4/32  unicast [ospf1 16:45:22.661] * I (150/10) [138.124.75.88]
      via 192.168.111.30 on wg_egw_eu2
      Type: OSPF univ
      OSPF.metric1: 10
      OSPF.router_id: 138.124.75.88
  ```
  `OSPF.router_id` is the advertising router, which is the field the map needs to attach a prefix
  to a site, alongside the next hop and outgoing interface.
- **A down peer vanishes from `show ospf neighbors`.** Observed live: `wg_egw_eu1` went from
  `Init/PtP` (in an earlier run) to absent. So the neighbor table is not a reliable list of
  configured links. The adjacency/edge table must be built **interface first** from
  `show ospf interface` (always lists every configured interface, here 7 rows), and enriched with
  neighbor identity when a row exists. `bird_ospf_interface_info` plus `bird_ospf_link_up`
  derived from presence/absence is the correct shape; a metric derived only from the neighbor
  table would silently drop broken links, which is exactly what the map must not do.
- State label values confirmed: `Full/BDR`, `Full/PtP`, `Init/PtP`. Expect `Down`, `Attempt`,
  `Init`, `2-Way`, `ExStart`, `Exchange`, `Loading`, `Full` crossed with roles `PtP`, `DR`,
  `BDR`, `DROther`.
- `DTime` behaves like the **remaining hold (dead) time**, not an elapsed counter: across three
  captures the values were 16.864-19.966, 18.216-19.027 and 15.363-19.092, always inside
  (dead - hello, dead] = (15, 20] for interfaces with `hello 5; dead 20`. Do not alert on it and
  do not label it "seconds since last change". One sample pair taken 1-2 s apart would confirm
  the direction (it should decrease).
- Column layout differs between one-shot `birdc "cmd"` (tabs) and an interactive terminal
  (space padded). The parser must split on any whitespace run, never on a fixed width.

### 8.5 Still missing

1. Multi-area or NSSA/stub confirmation. `xnetwork`, `xrouter` and `0003`/`0007` LSA rows are
   untested by the parser. Production looks single-area for now.
2. An interface in `Waiting` **without** `stub` (a real half-dead segment), and a neighbor in
   `2-Way`/`ExStart`/`Exchange`.
3. `DTime` direction confirmation (see 8.4), only if we decide to export it at all.
4. Optional, high value: the same capture from the MikroTik side or from another BIRD site (the
   `wg_ck` peer `192.168.112.6`) so per-vantage-point views can be cross-checked and one-way
   links detected. Note `wg_ck` peers a MikroTik; mktxp at `vic.fakecake.org` covers the device
   side, but the BIRD side of that link is already visible from this host, which is enough for
   the adjacency graph.

Note on `show route protocol ospf1` (present in the capture): each route carries
`(preference/metric)` and a trailing `[<router id>]` (the advertising/next-hop router) plus
`via <ip> on <iface>` or `dev <iface>`. That is a cheap and very useful prefix-to-site-to-interface
source, worth scraping behind an `-ospf.routes` flag.

## 9. Raw captures used

- `test-bird.txt` - egw-spb1, full command set, five adjacencies, healthy.
- `test-bird2.txt` - egw-spb1, `show status` plus `show route all protocol ospf1`.
- Inline samples in the chat: neighbor table with `Init/PtP`, then the same table with
  `wg_egw_eu1` missing (peer down).

These two files are working data and are not committed. The fixtures that the tests use are
anonymised copies of them, stored in `metrics/ospf_captures_test.go` with the addresses replaced
by RFC 5737/5738 documentation ranges and the host names by `example.net`.

## 10. What the first implementation ships

Implemented on the `dev` branch, verified end to end against a live BIRD 2.17.5 socket in a
network namespace and against the production captures above.

| Group | Flags (default) | Metrics |
|-------|-----------------|---------|
| Interfaces | `-ospf.interface` (on) | `bird_ospf_interface_info`, `bird_ospf_interface_cost`, `bird_ospf_interface_dr_info` |
| Adjacencies | `-ospf.neighbors` (on) | `bird_ospf_link_up`, `bird_ospf_link_info`, `bird_ospf_neighbor_state`, `bird_ospf_neighbor_hold_timer_seconds` |
| LSDB | `-ospf.lsdb` (off), `-ospf.lsdb.details` (off) | `bird_ospf_lsa_count`, plus `bird_ospf_lsa_age_seconds`, `bird_ospf_lsa_sequence` |
| SPF | `-ospf.topology` (off), `-ospf.topology.all` (off) | `bird_ospf_spf_distance`, `bird_ospf_spf_node_metric`, `bird_ospf_spf_network_router` |
| Routes | `-ospf.routes` (off) | `bird_ospf_route_info`, `bird_ospf_route_metric1`, `bird_ospf_route_best` |
| Daemon | always | `bird_daemon_info{router_id,version,hostname}` |

Measured on `egw-spb1` (7 interfaces, 5 neighbors, 30 LSAs, 30 routes):

- default flags (`-ospf.interface -ospf.neighbors`): 37 series per scrape;
- plus `-ospf.lsdb -ospf.topology -ospf.routes`: 161 series per scrape;
- plus `-ospf.lsdb.details -ospf.topology.all`: 221 series per scrape.

Six socket queries per scrape at default flags (`show protocols all`, `show ospf`, two `show
status`, `show ospf interface`, `show ospf neighbors`), nine with every group enabled. One of the
two `show status` calls feeds the `local_router_id` label and is not cached yet.

Deliberate limits of this first step, all of them cheap to lift later:

- only the new metric format (`-format.new=true`); the legacy `ospfv2_*` format is skipped;
- one `show status` query per OSPF protocol per scrape for `local_router_id`, no cache yet;
- no TTL cache of the parser results yet - a scrape that enables several groups queries each once;
- BIRD 1.x syntax for the new commands is not supported, nothing in the fleet runs 1.x;
- BIRD 3.x output was compared by hand for every command used and matches, but it is not in CI yet.

Follow-ups worth doing once the routing map exists:

- `min_ttl` / link-state ageing for `bird_ospf_link_up` and `bird_ospf_neighbor_state` (an
  adjacency is declared dead on the router after `dead`, but a scrape can miss a flap entirely);
- a `bird_ospf_area_info{name,area,area_id}` bridge metric, because the new metrics print the area
  as `0.0.0.0` and the older `bird_ospf_*_count` metrics print `0`;
- per-protocol scrape duration and query error metrics, which the generic exporter does not have;
- NBMA: `show ospf state` per neighbor is needed for the neighbor state when an interface is NBMA.
