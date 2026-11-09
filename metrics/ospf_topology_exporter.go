package metrics

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/czerwonk/bird_exporter/client"
	"github.com/czerwonk/bird_exporter/protocol"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

// OSPFTopologyOptions selects which OSPF topology groups are exported.
//
// The groups map one-to-one onto BIRD commands, so a deployment can enable the
// cheap ones (interfaces, neighbors) everywhere and the larger ones (lsdb,
// topology, routes) only on dedicated collector instances.
type OSPFTopologyOptions struct {
	Interfaces  bool
	Neighbors   bool
	LSDB        bool
	LSDBDetail  bool
	Topology    bool
	TopologyAll bool
	Routes      bool
}

func (o OSPFTopologyOptions) anyEnabled() bool {
	return o.Interfaces || o.Neighbors || o.LSDB || o.LSDBDetail || o.Topology || o.Routes
}

// seriesGuard drops a series whose full label set was already emitted for this
// protocol instance. BIRD can print the same entry twice, and one duplicated
// label set makes the Prometheus registry reject the whole scrape, which would
// take every other metric down with it.
type seriesGuard struct {
	keys map[string]struct{}
}

func newSeriesGuard() *seriesGuard {
	return &seriesGuard{keys: map[string]struct{}{}}
}

func (g *seriesGuard) set(ch chan<- prometheus.Metric, desc *prometheus.Desc, value float64,
	labels ...string,
) {
	key := fmt.Sprintf("%p", desc) + "\x00" + strings.Join(labels, "\x00")
	if _, duplicate := g.keys[key]; duplicate {
		log.Warnf("skipping duplicate OSPF metric %s with labels %v", desc.String(), labels)

		return
	}

	g.keys[key] = struct{}{}
	ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, value, labels...)
}

// neighborStateValues maps the OSPF finite state machine to numbers, in the
// order BIRD walks it during adjacency formation. Unknown states are -1 so
// they cannot be mistaken for "Down".
var neighborStateValues = map[string]float64{
	"Down":     0,
	"Attempt":  1,
	"Init":     2,
	"2-Way":    3,
	"ExStart":  4,
	"Exchange": 5,
	"Loading":  6,
	"Full":     7,
}

type ospfTopologyDesc struct {
	interfaceInfo     *prometheus.Desc
	interfaceCost     *prometheus.Desc
	interfaceDRInfo   *prometheus.Desc
	linkUp            *prometheus.Desc
	linkInfo          *prometheus.Desc
	neighborState     *prometheus.Desc
	neighborHoldTimer *prometheus.Desc
	lsaCount          *prometheus.Desc
	lsaAge            *prometheus.Desc
	lsaSequence       *prometheus.Desc
	spfDistance       *prometheus.Desc
	spfNodeMetric     *prometheus.Desc
	spfNetworkRouter  *prometheus.Desc
	routeInfo         *prometheus.Desc
	routeMetric       *prometheus.Desc
	routeBest         *prometheus.Desc
}

type ospfTopologyMetricExporter struct {
	prefix       string
	client       client.Client
	options      OSPFTopologyOptions
	descriptions *ospfTopologyDesc
}

// NewOSPFTopologyExporter creates a MetricExporter for OSPF topology and
// protocol status details: interfaces, adjacencies, link state database, SPF
// result and OSPF routes.
func NewOSPFTopologyExporter(prefix string, c client.Client, options OSPFTopologyOptions) MetricExporter {
	l := []string{"name"}
	a := []string{"name", "area"}

	d := &ospfTopologyDesc{}

	d.interfaceInfo = prometheus.NewDesc(prefix+"ospf_interface_info",
		"OSPF interface (1 = present); local IPv4 network and area are labels",
		append(a, "iface", "network", "type", "state", "stub"), nil)
	d.interfaceCost = prometheus.NewDesc(prefix+"ospf_interface_cost",
		"Configured OSPF cost of the interface", append(a, "iface"), nil)
	d.interfaceDRInfo = prometheus.NewDesc(prefix+"ospf_interface_dr_info",
		"Designated router of an OSPF broadcast or NBMA segment",
		append(a, "iface", "dr_id", "dr_ip", "bdr_id", "bdr_ip"), nil)

	d.linkUp = prometheus.NewDesc(prefix+"ospf_link_up",
		"Adjacency state of a configured OSPF link: 1 = Full, 0 = not adjacent",
		append(a, "local_router_id", "local_iface", "local_network"), nil)
	d.linkInfo = prometheus.NewDesc(prefix+"ospf_link_info",
		"OSPF adjacency: local interface and router paired with neighbor router ID and interface IP",
		append(a, "local_router_id", "local_iface", "local_network", "remote_router_id",
			"remote_ip", "state", "role", "priority"), nil)
	d.neighborState = prometheus.NewDesc(prefix+"ospf_neighbor_state",
		"OSPF neighbor finite state machine: 0=Down 1=Attempt 2=Init 3=2-Way 4=ExStart 5=Exchange 6=Loading 7=Full, -1=unknown",
		append(l, "area", "iface", "neighbor_id", "neighbor_ip"), nil)
	d.neighborHoldTimer = prometheus.NewDesc(prefix+"ospf_neighbor_hold_timer_seconds",
		"Remaining OSPF hold (dead) timer of the neighbor, reset to the interface dead timer on every hello",
		append(l, "area", "iface", "neighbor_id", "neighbor_ip"), nil)

	d.lsaCount = prometheus.NewDesc(prefix+"ospf_lsa_count",
		"Number of LSAs in the OSPF link state database, area is empty for the global table",
		append(l, "area", "type"), nil)
	d.lsaAge = prometheus.NewDesc(prefix+"ospf_lsa_age_seconds",
		"Age of an LSA in the OSPF link state database",
		append(l, "area", "type", "ls_id", "router"), nil)
	d.lsaSequence = prometheus.NewDesc(prefix+"ospf_lsa_sequence",
		"Sequence number of an LSA as decimal, increases while the LSA is re-originated",
		append(l, "area", "type", "ls_id", "router"), nil)

	d.spfDistance = prometheus.NewDesc(prefix+"ospf_spf_distance",
		"SPF distance from a router in the area to itself (0 for the local router)",
		append(l, "area", "root", "root_kind"), nil)
	d.spfNodeMetric = prometheus.NewDesc(prefix+"ospf_spf_node_metric",
		"SPF cost from a root router to a node of its tree (minimum if several paths exist)",
		append(l, "area", "root", "kind", "peer", "prefix", "metric_type"), nil)
	d.spfNetworkRouter = prometheus.NewDesc(prefix+"ospf_spf_network_router",
		"Router attached to a transit network node of the SPF tree",
		append(l, "area", "network", "router", "dr"), nil)

	d.routeInfo = prometheus.NewDesc(prefix+"ospf_route_info",
		"Route imported by an OSPF protocol with advertising router and next hop",
		append(l, "prefix", "advertised_by", "nexthop_ip", "iface", "type"), nil)
	d.routeMetric = prometheus.NewDesc(prefix+"ospf_route_metric1",
		"OSPF metric1 of an imported route",
		append(l, "prefix", "advertised_by", "nexthop_ip", "iface"), nil)
	d.routeBest = prometheus.NewDesc(prefix+"ospf_route_best",
		"Whether the imported route is the selected one: 1 = best",
		append(l, "prefix", "advertised_by", "nexthop_ip", "iface"), nil)

	return &ospfTopologyMetricExporter{prefix: prefix, client: c, options: options, descriptions: d}
}

func (m *ospfTopologyMetricExporter) Describe(ch chan<- *prometheus.Desc) {
	d := m.descriptions
	o := m.options

	if o.Interfaces {
		ch <- d.interfaceInfo
		ch <- d.interfaceCost
		ch <- d.interfaceDRInfo
	}

	if o.Neighbors {
		ch <- d.linkUp
		ch <- d.linkInfo
		ch <- d.neighborState
		ch <- d.neighborHoldTimer
	}

	if o.LSDB {
		ch <- d.lsaCount
	}

	if o.LSDBDetail {
		ch <- d.lsaAge
		ch <- d.lsaSequence
	}

	if o.Topology {
		ch <- d.spfDistance
		ch <- d.spfNodeMetric
		ch <- d.spfNetworkRouter
	}

	if o.Routes {
		ch <- d.routeInfo
		ch <- d.routeMetric
		ch <- d.routeBest
	}
}

// Export collects the OSPF topology groups for a single protocol instance.
//
// Link metrics are emitted interface first: every interface BIRD has OSPF
// enabled on produces a bird_ospf_link_up series. A peer that stopped sending
// hellos disappears from "show ospf neighbors", but the interface stays, so a
// broken link shows up as link_up = 0 instead of vanishing.
func (m *ospfTopologyMetricExporter) Export(p *protocol.Protocol, ch chan<- prometheus.Metric, newFormat bool) {
	if !m.options.anyEnabled() {
		return
	}

	if !newFormat {
		log.Debugln("OSPF topology metrics require the new metric format, skipping")

		return
	}

	guard := newSeriesGuard()

	localRouterID, err := m.client.GetRouterID(p)
	if err != nil {
		log.Errorf("could not get router ID for OSPF protocol %q: %v", p.Name, err)
	}

	needInterfaces := m.options.Interfaces || m.options.Neighbors

	var (
		interfaces []*protocol.OSPFInterface
		neighbors  []*protocol.OSPFNeighbor
	)

	if needInterfaces {
		interfaces, err = m.client.GetOSPFInterfaces(p)
		if err != nil {
			log.Errorf("could not get OSPF interfaces for protocol %q: %v", p.Name, err)
		}

		if m.options.Neighbors {
			neighbors, err = m.client.GetOSPFNeighbors(p)
			if err != nil {
				log.Errorf("could not get OSPF neighbors for protocol %q: %v", p.Name, err)
			}
		}
	}

	byInterface := make(map[string]*protocol.OSPFInterface, len(interfaces))
	for _, iface := range interfaces {
		byInterface[iface.Name] = iface
	}

	if m.options.Interfaces {
		for _, iface := range interfaces {
			m.exportInterface(p, iface, ch, guard)
		}
	}

	if m.options.Neighbors {
		adjacency := make(map[string]*protocol.OSPFNeighbor, len(neighbors))
		for _, neighbor := range neighbors {
			neighbor.Area = areaOf(byInterface, neighbor.Interface)
			adjacency[neighbor.Interface] = neighbor
			m.exportNeighbor(p, neighbor, ch, guard)
		}

		for _, iface := range interfaces {
			m.exportLink(p, localRouterID, iface, adjacency[iface.Name], ch, guard)
		}
	}

	if m.options.LSDB || m.options.LSDBDetail {
		lsas, err := m.client.GetOSPFLSDB(p)
		if err != nil {
			log.Errorf("could not get OSPF link state database for protocol %q: %v", p.Name, err)
		} else {
			m.exportLSDB(p, lsas, ch, guard)
		}
	}

	if m.options.Topology {
		roots, err := m.client.GetOSPFTopology(p, m.options.TopologyAll)
		if err != nil {
			log.Errorf("could not get OSPF topology for protocol %q: %v", p.Name, err)
		} else {
			m.exportTopology(p, roots, ch, guard)
		}
	}

	if m.options.Routes {
		routes, err := m.client.GetOSPFRoutes(p)
		if err != nil {
			log.Errorf("could not get OSPF routes for protocol %q: %v", p.Name, err)
		} else {
			m.exportRoutes(p, routes, ch, guard)
		}
	}
}

func (m *ospfTopologyMetricExporter) exportInterface(p *protocol.Protocol, iface *protocol.OSPFInterface,
	ch chan<- prometheus.Metric, guard *seriesGuard,
) {
	d := m.descriptions
	area := iface.Area

	guard.set(ch, d.interfaceInfo, 1,
		p.Name, area, iface.Name, iface.Network, iface.Type, iface.State, boolLabel(iface.Stub))

	guard.set(ch, d.interfaceCost, float64(iface.Cost), p.Name, area, iface.Name)

	if iface.DRID != "" || iface.DRIp != "" {
		guard.set(ch, d.interfaceDRInfo, 1,
			p.Name, area, iface.Name, iface.DRID, iface.DRIp, iface.BDRID, iface.BDRIp)
	}
}

func (m *ospfTopologyMetricExporter) exportNeighbor(p *protocol.Protocol, neighbor *protocol.OSPFNeighbor,
	ch chan<- prometheus.Metric, guard *seriesGuard,
) {
	d := m.descriptions

	state, known := neighborStateValues[neighbor.State]
	if !known {
		log.Warnf("unknown OSPF neighbor state %q for neighbor %s on %s", neighbor.State,
			neighbor.RouterID, neighbor.Interface)
		state = -1
	}

	labels := []string{p.Name, neighbor.Area, neighbor.Interface, neighbor.RouterID, neighbor.NeighborIP}
	guard.set(ch, d.neighborState, state, labels...)
	guard.set(ch, d.neighborHoldTimer, neighbor.HoldTimer, labels...)
}

func (m *ospfTopologyMetricExporter) exportLink(p *protocol.Protocol, localRouterID string,
	iface *protocol.OSPFInterface, neighbor *protocol.OSPFNeighbor, ch chan<- prometheus.Metric,
	guard *seriesGuard,
) {
	d := m.descriptions
	base := []string{p.Name, iface.Area, localRouterID, iface.Name, iface.Network}

	up := float64(0)
	if neighbor != nil && neighbor.Adjacent() {
		up = 1
	}

	guard.set(ch, d.linkUp, up, base...)

	if neighbor == nil {
		return
	}

	guard.set(ch, d.linkInfo, 1,
		append(base, neighbor.RouterID, neighbor.NeighborIP, neighbor.State, neighbor.Role,
			fmt.Sprintf("%d", neighbor.Priority))...)
}

func (m *ospfTopologyMetricExporter) exportLSDB(p *protocol.Protocol, lsas []*protocol.OSPFLSA,
	ch chan<- prometheus.Metric, guard *seriesGuard,
) {
	d := m.descriptions

	type lsaKey struct {
		area string
		kind string
	}

	counts := map[lsaKey]int64{}
	order := make([]lsaKey, 0, len(lsas))

	for _, lsa := range lsas {
		key := lsaKey{area: lsa.Area, kind: lsa.Type}
		if _, seen := counts[key]; !seen {
			order = append(order, key)
		}

		counts[key]++
	}

	for _, key := range order {
		guard.set(ch, d.lsaCount, float64(counts[key]), p.Name, key.area, key.kind)
	}

	if !m.options.LSDBDetail {
		return
	}

	for _, lsa := range lsas {
		labels := []string{p.Name, lsa.Area, lsa.Type, lsa.LSID, lsa.Router}
		guard.set(ch, d.lsaAge, float64(lsa.Age), labels...)
		guard.set(ch, d.lsaSequence, float64(parseHexSequence(lsa.Sequence)), labels...)
	}
}

func (m *ospfTopologyMetricExporter) exportTopology(p *protocol.Protocol, roots []*protocol.OSPFSpfRoot,
	ch chan<- prometheus.Metric, guard *seriesGuard,
) {
	d := m.descriptions

	for _, root := range roots {
		guard.set(ch, d.spfDistance, float64(root.Distance), p.Name, root.Area, root.Identity, root.Kind)

		for _, router := range root.Routers {
			guard.set(ch, d.spfNetworkRouter, 1, p.Name, root.Area, root.Identity, router, root.DR)
		}

		type nodeKey struct {
			kind       string
			peer       string
			prefix     string
			metricType string
		}

		best := map[nodeKey]int64{}
		order := make([]nodeKey, 0, len(root.Nodes))

		for _, node := range root.Nodes {
			key := nodeKey{kind: node.Kind, peer: node.Peer, prefix: node.Prefix, metricType: node.MetricType}
			previous, seen := best[key]
			if !seen {
				order = append(order, key)
				best[key] = node.Metric

				continue
			}

			if node.Metric < previous {
				best[key] = node.Metric
			}
		}

		for _, key := range order {
			guard.set(ch, d.spfNodeMetric, float64(best[key]), p.Name, root.Area, root.Identity,
				key.kind, key.peer, key.prefix, key.metricType)
		}
	}
}

func (m *ospfTopologyMetricExporter) exportRoutes(p *protocol.Protocol, routes []*protocol.OSPFRoute,
	ch chan<- prometheus.Metric, guard *seriesGuard,
) {
	d := m.descriptions

	for _, route := range routes {
		labels := []string{p.Name, route.Prefix, route.AdvertisedBy, route.NextHopIP, route.Interface}

		guard.set(ch, d.routeInfo, 1, append(labels, route.Type)...)
		guard.set(ch, d.routeMetric, float64(route.Metric1), labels...)

		best := float64(0)
		if route.Best {
			best = 1
		}

		guard.set(ch, d.routeBest, best, labels...)
	}
}

// areaOf returns the area of an interface name, empty when unknown.
func areaOf(interfaces map[string]*protocol.OSPFInterface, name string) string {
	if iface, ok := interfaces[name]; ok {
		return iface.Area
	}

	return ""
}

// parseHexSequence converts a BIRD LSA sequence number (hex, e.g. 8000004e)
// to a number. Values above 2^31 are rare but fit into int64.
func parseHexSequence(value string) int64 {
	parsed, err := strconv.ParseInt(value, 16, 64)
	if err != nil {
		log.Debugf("could not parse LSA sequence %q: %v", value, err)

		return 0
	}

	return parsed
}

// boolLabel renders a boolean as a stable label value.
func boolLabel(value bool) string {
	if value {
		return "yes"
	}

	return "no"
}
