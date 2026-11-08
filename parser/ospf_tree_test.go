package parser

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// topologyLive is "show ospf topology" output of a production router, reduced
// to two routers and one transit network, with addresses replaced by
// documentation ranges.
const topologyLive = "\narea 0.0.0.0\n\n" +
	"\trouter 203.0.113.3\n" +
	"\t\tdistance 10\n" +
	"\t\trouter 203.0.113.1 metric 10\n" +
	"\t\tstubnet 192.0.2.52/30 metric 10\n" +
	"\t\texternal 192.0.2.12/30 metric2 10000\n" +
	"\n\trouter 203.0.113.1\n" +
	"\t\tdistance 0\n" +
	"\t\trouter 198.51.100.5 metric 10\n" +
	"\t\trouter 203.0.113.3 metric 10\n" +
	"\t\trouter 203.0.113.3 metric 15\n" +
	"\t\tnetwork 198.51.100.128/28 metric 10\n" +
	"\t\tstubnet 198.51.100.3/32 metric 0\n" +
	"\t\txnetwork 198.51.110.0/24 metric 25\n" +
	"\n\tnetwork 198.51.100.128/28\n" +
	"\t\tdr 203.0.113.1\n" +
	"\t\tdistance 10\n" +
	"\t\trouter 203.0.113.1\n" +
	"\t\trouter 203.0.113.2\n"

func TestParseOSPFTopology(t *testing.T) {
	roots, err := ParseOSPFTopologyWithError([]byte(topologyLive))
	require.NoError(t, err)
	require.Len(t, roots, 3)

	remote := roots[0]
	assert.Equal(t, "0.0.0.0", remote.Area)
	assert.Equal(t, "router", remote.Kind)
	assert.Equal(t, "203.0.113.3", remote.Identity)
	assert.Equal(t, int64(10), remote.Distance)
	require.Len(t, remote.Nodes, 3)

	peer := remote.Nodes[0]
	assert.Equal(t, "router", peer.Kind)
	assert.Equal(t, "203.0.113.1", peer.Peer)
	assert.Empty(t, peer.Prefix)
	assert.Equal(t, int64(10), peer.Metric)
	assert.Empty(t, peer.MetricType)

	stubnet := remote.Nodes[1]
	assert.Equal(t, "stubnet", stubnet.Kind)
	assert.Equal(t, "192.0.2.52/30", stubnet.Prefix)

	external := remote.Nodes[2]
	assert.Equal(t, "external", external.Kind)
	assert.Equal(t, "192.0.2.12/30", external.Prefix)
	assert.Equal(t, "2", external.MetricType)
	assert.Equal(t, int64(10000), external.Metric)

	local := roots[1]
	assert.Equal(t, int64(0), local.Distance)
	require.Len(t, local.Nodes, 6)

	// The parser keeps every path BIRD prints; the exporter picks the minimum.
	assert.Equal(t, int64(10), local.Nodes[1].Metric)
	assert.Equal(t, int64(15), local.Nodes[2].Metric)

	interArea := local.Nodes[5]
	assert.Equal(t, "xnetwork", interArea.Kind)
	assert.Equal(t, "198.51.110.0/24", interArea.Prefix)

	network := roots[2]
	assert.Equal(t, "network", network.Kind)
	assert.Equal(t, "198.51.100.128/28", network.Identity)
	assert.Equal(t, "203.0.113.1", network.DR)
	assert.Equal(t, int64(10), network.Distance)
	assert.Equal(t, []string{"203.0.113.1", "203.0.113.2"}, network.Routers)
	assert.Empty(t, network.Nodes)
}
