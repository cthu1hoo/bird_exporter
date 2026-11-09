package metrics

import (
	"errors"
	"testing"

	"github.com/czerwonk/bird_exporter/client"
	"github.com/czerwonk/bird_exporter/parser"
	"github.com/czerwonk/bird_exporter/protocol"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeClient returns canned BIRD replies for the OSPF topology exporter tests.
var _ client.Client = (*fakeClient)(nil)

type fakeClient struct {
	routerID   string
	interfaces []*protocol.OSPFInterface
	neighbors  []*protocol.OSPFNeighbor
	lsas       []*protocol.OSPFLSA
	roots      []*protocol.OSPFSpfRoot
	routes     []*protocol.OSPFRoute
	failRoutes bool
}

func (f *fakeClient) GetProtocols() ([]*protocol.Protocol, error) { return nil, nil }

func (f *fakeClient) GetOSPFAreas(*protocol.Protocol) ([]*protocol.OSPFArea, error) { return nil, nil }

func (f *fakeClient) GetBFDSessions(*protocol.Protocol) ([]*protocol.BFDSession, error) {
	return nil, nil
}

func (f *fakeClient) GetRouterID(*protocol.Protocol) (string, error) { return f.routerID, nil }

func (f *fakeClient) GetOSPFInterfaces(*protocol.Protocol) ([]*protocol.OSPFInterface, error) {
	return f.interfaces, nil
}

func (f *fakeClient) GetOSPFNeighbors(*protocol.Protocol) ([]*protocol.OSPFNeighbor, error) {
	return f.neighbors, nil
}

func (f *fakeClient) GetOSPFLSDB(*protocol.Protocol) ([]*protocol.OSPFLSA, error) { return f.lsas, nil }

func (f *fakeClient) GetOSPFTopology(*protocol.Protocol, bool) ([]*protocol.OSPFSpfRoot, error) {
	return f.roots, nil
}

func (f *fakeClient) GetOSPFRoutes(*protocol.Protocol) ([]*protocol.OSPFRoute, error) {
	if f.failRoutes {
		return nil, errors.New("query failed")
	}

	return f.routes, nil
}

// gather runs the exporter through a registry so metric names and labels can be
// inspected the same way Prometheus sees them.
func gather(t *testing.T, exporter MetricExporter, p *protocol.Protocol) map[string]*dto.MetricFamily {
	t.Helper()

	registry := prometheus.NewRegistry()
	require.NoError(t, registry.Register(&singleProtocolCollector{exporter: exporter, proto: p}))

	families, err := registry.Gather()
	require.NoError(t, err)

	byName := make(map[string]*dto.MetricFamily, len(families))
	for _, family := range families {
		byName[family.GetName()] = family
	}

	return byName
}

type singleProtocolCollector struct {
	exporter MetricExporter
	proto    *protocol.Protocol
}

func (c *singleProtocolCollector) Describe(ch chan<- *prometheus.Desc) { c.exporter.Describe(ch) }

func (c *singleProtocolCollector) Collect(ch chan<- prometheus.Metric) {
	c.exporter.Export(c.proto, ch, true)
}

func labelsOf(metric *dto.Metric) map[string]string {
	labels := map[string]string{}
	for _, label := range metric.GetLabel() {
		labels[label.GetName()] = label.GetValue()
	}

	return labels
}

func ospfFixtureClient() *fakeClient {
	return &fakeClient{
		routerID: "203.0.113.1",
		interfaces: []*protocol.OSPFInterface{
			{
				Name: "eth1", Network: "198.51.100.128/28", Type: "broadcast", Area: "0.0.0.0",
				AreaID: "0", State: "DR", Cost: 10, DRID: "203.0.113.1", DRIp: "198.51.100.133",
				BDRID: "203.0.113.2", BDRIp: "198.51.100.132",
			},
			{Name: "wg_ck", Network: "192.0.2.8/30", Type: "ptp", Area: "0.0.0.0", State: "PtP", Cost: 15},
			{Name: "wg_server", Network: "192.0.2.192/28", Type: "ptp", Area: "0.0.0.0", State: "PtP", Cost: 10},
			{Name: "br_float", Network: "198.51.100.3/32", Type: "broadcast", Area: "0.0.0.0", State: "Waiting", Stub: true, Cost: 10},
		},
		neighbors: []*protocol.OSPFNeighbor{
			{
				RouterID: "203.0.113.2", Priority: 1, State: "Full", Role: "BDR", HoldTimer: 15.594,
				Interface: "eth1", NeighborIP: "198.51.100.132",
			},
			{
				RouterID: "198.51.100.6", Priority: 128, State: "Init", Role: "PtP", HoldTimer: 18.349,
				Interface: "wg_ck", NeighborIP: "192.0.2.10",
			},
		},
		lsas: []*protocol.OSPFLSA{
			{Area: "0.0.0.0", Type: "0001", LSID: "203.0.113.1", Router: "203.0.113.1", Sequence: "800019e5", Age: 284},
			{Area: "0.0.0.0", Type: "0001", LSID: "203.0.113.2", Router: "203.0.113.2", Sequence: "8000a411", Age: 286},
			{Area: "0.0.0.0", Type: "0002", LSID: "198.51.100.133", Router: "203.0.113.1", Sequence: "8000019b", Age: 1390},
			{Area: "", Type: "0005", LSID: "192.0.2.12", Router: "203.0.113.3", Sequence: "8000004e", Age: 299},
		},
		roots: []*protocol.OSPFSpfRoot{
			{
				Area: "0.0.0.0", Kind: "router", Identity: "203.0.113.1", Distance: 0,
				Nodes: []protocol.OSPFSpfNode{
					{Kind: "router", Peer: "203.0.113.2", Metric: 10},
					{Kind: "router", Peer: "203.0.113.2", Metric: 15},
					{Kind: "stubnet", Prefix: "198.51.100.3/32", Metric: 0},
				},
			},
			{
				Area: "0.0.0.0", Kind: "network", Identity: "198.51.100.128/28", Distance: 10, DR: "203.0.113.1",
				Routers: []string{"203.0.113.1", "203.0.113.2"},
			},
		},
		routes: []*protocol.OSPFRoute{
			{
				Prefix: "192.0.2.8/30", AdvertisedBy: "203.0.113.1", Metric1: 10, Type: "OSPF univ",
				Interface: "wg_ck", Best: true,
			},
			{
				Prefix: "198.51.100.4/32", AdvertisedBy: "203.0.113.4", Metric1: 25, Type: "OSPF univ",
				NextHopIP: "192.0.2.30", Interface: "wg_egw_eu2",
			},
		},
	}
}

func ospfProtocol() *protocol.Protocol {
	return protocol.NewProtocol("ospf1", protocol.OSPF, "4", 100)
}

func allOptionsEnabled() OSPFTopologyOptions {
	return OSPFTopologyOptions{
		Interfaces: true, Neighbors: true, LSDB: true, LSDBDetail: true,
		Topology: true, TopologyAll: true, Routes: true,
	}
}

func TestOSPFTopologyExporterLinksAreInterfaceFirst(t *testing.T) {
	c := ospfFixtureClient()
	exporter := NewOSPFTopologyExporter("bird_", c, OSPFTopologyOptions{Interfaces: true, Neighbors: true})

	families := gather(t, exporter, ospfProtocol())

	linkUp := families["bird_ospf_link_up"]
	require.NotNil(t, linkUp)
	require.Len(t, linkUp.GetMetric(), 4)

	wantUp := map[string]float64{
		"eth1":      1,
		"wg_ck":     0,
		"wg_server": 0,
		"br_float":  0,
	}

	gotUp := map[string]float64{}
	for _, metric := range linkUp.GetMetric() {
		labels := labelsOf(metric)
		assert.Equal(t, "203.0.113.1", labels["local_router_id"])
		gotUp[labels["local_iface"]] = metric.GetGauge().GetValue()
	}

	assert.Equal(t, wantUp, gotUp)

	linkInfo := families["bird_ospf_link_info"]
	require.NotNil(t, linkInfo)
	require.Len(t, linkInfo.GetMetric(), 2)

	for _, metric := range linkInfo.GetMetric() {
		labels := labelsOf(metric)
		assert.NotEmpty(t, labels["remote_router_id"])
		assert.NotEmpty(t, labels["remote_ip"])
		assert.NotEmpty(t, labels["local_network"])
	}

	// Only the broadcast segment has a designated router.
	drInfo := families["bird_ospf_interface_dr_info"]
	require.NotNil(t, drInfo)
	require.Len(t, drInfo.GetMetric(), 1)
	assert.Equal(t, "eth1", labelsOf(drInfo.GetMetric()[0])["iface"])

	// The stub interface flag survives as a label.
	for _, metric := range families["bird_ospf_interface_info"].GetMetric() {
		labels := labelsOf(metric)
		if labels["iface"] == "br_float" {
			assert.Equal(t, "yes", labels["stub"])
			assert.Equal(t, "Waiting", labels["state"])
		}
	}
}

func TestOSPFTopologyExporterNeighborStates(t *testing.T) {
	c := ospfFixtureClient()
	exporter := NewOSPFTopologyExporter("bird_", c, OSPFTopologyOptions{Neighbors: true})

	families := gather(t, exporter, ospfProtocol())

	states := families["bird_ospf_neighbor_state"]
	require.NotNil(t, states)
	require.Len(t, states.GetMetric(), 2)

	byNeighbour := map[string]float64{}
	for _, metric := range states.GetMetric() {
		byNeighbour[labelsOf(metric)["neighbor_id"]] = metric.GetGauge().GetValue()
	}

	assert.Equal(t, float64(7), byNeighbour["203.0.113.2"])
	assert.Equal(t, float64(2), byNeighbour["198.51.100.6"])

	hold := families["bird_ospf_neighbor_hold_timer_seconds"]
	require.NotNil(t, hold)
	assert.InDelta(t, 15.594, hold.GetMetric()[0].GetGauge().GetValue(), 0.0001)
}

func TestOSPFTopologyExporterTopologyAndDatabase(t *testing.T) {
	c := ospfFixtureClient()
	exporter := NewOSPFTopologyExporter("bird_", c, OSPFTopologyOptions{LSDB: true, Topology: true})

	families := gather(t, exporter, ospfProtocol())

	counts := families["bird_ospf_lsa_count"]
	require.NotNil(t, counts)

	byType := map[string]float64{}
	for _, metric := range counts.GetMetric() {
		labels := labelsOf(metric)
		byType[labels["area"]+"/"+labels["type"]] = metric.GetGauge().GetValue()
	}

	assert.Equal(t, float64(2), byType["0.0.0.0/0001"])
	assert.Equal(t, float64(1), byType["0.0.0.0/0002"])
	assert.Equal(t, float64(1), byType["/0005"])

	nodes := families["bird_ospf_spf_node_metric"]
	require.NotNil(t, nodes)

	// The duplicate path to 203.0.113.2 collapses to the cheapest metric.
	for _, metric := range nodes.GetMetric() {
		labels := labelsOf(metric)
		if labels["kind"] == "router" && labels["peer"] == "203.0.113.2" {
			assert.Equal(t, float64(10), metric.GetGauge().GetValue())
		}
	}

	networkRouters := families["bird_ospf_spf_network_router"]
	require.NotNil(t, networkRouters)
	require.Len(t, networkRouters.GetMetric(), 2)

	// Per-LSA details stay behind their own flag.
	assert.Nil(t, families["bird_ospf_lsa_age_seconds"])
}

func TestOSPFTopologyExporterRoutesAndDisabledGroups(t *testing.T) {
	c := ospfFixtureClient()
	exporter := NewOSPFTopologyExporter("bird_", c, OSPFTopologyOptions{Routes: true})

	families := gather(t, exporter, ospfProtocol())

	require.Len(t, families["bird_ospf_route_info"].GetMetric(), 2)
	require.Len(t, families["bird_ospf_route_metric1"].GetMetric(), 2)
	require.Len(t, families["bird_ospf_route_best"].GetMetric(), 2)
	assert.Nil(t, families["bird_ospf_interface_info"])
	assert.Nil(t, families["bird_ospf_link_up"])

	for _, metric := range families["bird_ospf_route_info"].GetMetric() {
		labels := labelsOf(metric)
		assert.NotEmpty(t, labels["prefix"])
		assert.NotEmpty(t, labels["advertised_by"])
		assert.Equal(t, "OSPF univ", labels["type"])
	}
}

func TestOSPFTopologyExporterSurvivesQueryErrors(t *testing.T) {
	c := ospfFixtureClient()
	c.failRoutes = true
	exporter := NewOSPFTopologyExporter("bird_", c, allOptionsEnabled())

	families := gather(t, exporter, ospfProtocol())

	require.Len(t, families["bird_ospf_interface_info"].GetMetric(), 4)
	assert.Nil(t, families["bird_ospf_route_info"])
}

// TestOSPFTopologyExporterSkipsDuplicateSeries guards the scrape: one duplicated
// label set makes the Prometheus registry reject every metric of the scrape,
// and BIRD has been seen to name the same SPF root twice.
func TestOSPFTopologyExporterSkipsDuplicateSeries(t *testing.T) {
	c := ospfFixtureClient()
	c.interfaces = []*protocol.OSPFInterface{
		{Name: "eth1", Area: "0.0.0.0", Network: "198.51.100.128/28", Type: "broadcast", State: "DR", Cost: 10},
		{Name: "eth1", Area: "0.0.0.0", Network: "198.51.100.128/28", Type: "broadcast", State: "DR", Cost: 10},
	}
	c.neighbors = []*protocol.OSPFNeighbor{
		{RouterID: "203.0.113.2", State: "Full", Role: "BDR", Interface: "eth1", NeighborIP: "198.51.100.132"},
		{RouterID: "203.0.113.2", State: "Full", Role: "BDR", Interface: "eth1", NeighborIP: "198.51.100.132"},
	}
	c.roots = []*protocol.OSPFSpfRoot{
		{Area: "0.0.0.0", Kind: "router", Identity: "203.0.113.1", Distance: 0},
		{Area: "0.0.0.0", Kind: "router", Identity: "203.0.113.1", Distance: 0},
		{Area: "0.0.0.0", Kind: "router", Identity: "203.0.113.3", Distance: 0},
		{Area: "0.0.0.0", Kind: "network", Identity: "198.51.100.128/28", Distance: 10,
			DR: "203.0.113.1", Routers: []string{"203.0.113.1", "203.0.113.1"}},
	}

	families := gather(t, NewOSPFTopologyExporter("bird_", c, allOptionsEnabled()), ospfProtocol())

	assert.Len(t, families["bird_ospf_interface_info"].GetMetric(), 1)
	assert.Len(t, families["bird_ospf_link_up"].GetMetric(), 1)
	assert.Len(t, families["bird_ospf_link_info"].GetMetric(), 1)
	assert.Len(t, families["bird_ospf_neighbor_state"].GetMetric(), 1)
	assert.Len(t, families["bird_ospf_spf_distance"].GetMetric(), 3)
	assert.Len(t, families["bird_ospf_spf_network_router"].GetMetric(), 1)
}

// TestOSPFTopologyExportersParseLiveCaptures feeds the recorded production
// output through the real parsers and the exporter, to make sure the metric
// shapes hold for a full adjacency set.
func TestOSPFTopologyExportersParseLiveCaptures(t *testing.T) {
	c := ospfFixtureClient()

	c.interfaces = parseFixture(t, parser.ParseOSPFInterfacesWithError, interfacesCapture)
	c.neighbors = parseFixture(t, parser.ParseOSPFNeighborsWithError, neighborsCapture)
	c.lsas = parseFixture(t, parser.ParseOSPFDatabaseWithError, lsadbCapture)
	c.roots = parseFixture(t, parser.ParseOSPFTopologyWithError, topologyCapture)
	c.routes = parseFixture(t, parser.ParseOSPFRoutesWithError, routesCapture)

	require.Len(t, c.interfaces, 7)
	require.Len(t, c.neighbors, 5)
	require.Len(t, c.lsas, 30)
	require.Len(t, c.roots, 7)
	require.Len(t, c.routes, 30)

	exporter := NewOSPFTopologyExporter("bird_", c, allOptionsEnabled())
	families := gather(t, exporter, ospfProtocol())

	assert.Equal(t, 7, len(families["bird_ospf_interface_info"].GetMetric()))
	assert.Equal(t, 5, len(families["bird_ospf_link_info"].GetMetric()))

	linkUp := map[string]float64{}
	for _, metric := range families["bird_ospf_link_up"].GetMetric() {
		linkUp[labelsOf(metric)["local_iface"]] = metric.GetGauge().GetValue()
	}

	assert.Equal(t, map[string]float64{
		"eth1": 1, "wg_ck": 1, "wg_home": 1, "wg_egw_eu1": 1, "wg_egw_eu2": 1,
		"wg_server": 0, "br_float": 0,
	}, linkUp)

	// Cardinality of the production data set, so an accidental series explosion
	// fails the build. Documented in docs/OSPF_TOPOLOGY_RESEARCH.md.
	assert.Equal(t, 37, countSeries(t, c, OSPFTopologyOptions{Interfaces: true, Neighbors: true}))
	assert.Equal(t, 221, countSeries(t, c, allOptionsEnabled()))
}

func countSeries(t *testing.T, c *fakeClient, options OSPFTopologyOptions) int {
	t.Helper()

	total := 0
	for _, family := range gather(t, NewOSPFTopologyExporter("bird_", c, options), ospfProtocol()) {
		total += len(family.GetMetric())
	}

	return total
}

func parseFixture[T any](t *testing.T, parse func([]byte) ([]T, error), fixture string) []T {
	t.Helper()

	parsed, err := parse([]byte(fixture))
	require.NoError(t, err)

	return parsed
}
