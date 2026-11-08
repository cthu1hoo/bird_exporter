package parser

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// routesLive is "show route all protocol ospf1" output of a production router,
// reduced and with addresses replaced by documentation ranges.
const routesLive = "Table master4:\n" +
	"198.51.100.4/32     unicast [ospf1 16:45:22.661] * I (150/10) [203.0.113.4]\n" +
	"\tvia 192.0.2.30 on wg_egw_eu2\n" +
	"\tType: OSPF univ\n" +
	"\tOSPF.metric1: 10\n" +
	"\tOSPF.router_id: 203.0.113.4\n" +
	"192.0.2.8/30     unicast [ospf1 2026-10-31] I (150/10) [203.0.113.1]\n" +
	"\tdev wg_ck\n" +
	"\tType: OSPF univ\n" +
	"\tOSPF.metric1: 10\n" +
	"\tOSPF.router_id: 203.0.113.1\n" +
	"198.51.100.128/28   unicast [ospf1 2026-10-31] * I (150/10) [203.0.113.1]\n" +
	"\tvia 198.51.100.133 on eth1\n" +
	"\tvia 198.51.100.134 on eth1\n" +
	"\tType: OSPF univ\n" +
	"\tOSPF.metric1: 10\n" +
	"\tOSPF.router_id: 203.0.113.1\n"

func TestParseOSPFRoutes(t *testing.T) {
	routes, err := ParseOSPFRoutesWithError([]byte(routesLive))
	require.NoError(t, err)
	require.Len(t, routes, 4)

	remote := routes[0]
	assert.Equal(t, "198.51.100.4/32", remote.Prefix)
	assert.Equal(t, "203.0.113.4", remote.AdvertisedBy)
	assert.Equal(t, "192.0.2.30", remote.NextHopIP)
	assert.Equal(t, "wg_egw_eu2", remote.Interface)
	assert.Equal(t, "OSPF univ", remote.Type)
	assert.Equal(t, int64(10), remote.Metric1)
	assert.True(t, remote.Best)

	direct := routes[1]
	assert.Equal(t, "192.0.2.8/30", direct.Prefix)
	assert.Empty(t, direct.NextHopIP)
	assert.Equal(t, "wg_ck", direct.Interface)
	assert.Equal(t, "203.0.113.1", direct.AdvertisedBy)
	assert.False(t, direct.Best)

	// Equal-cost paths are reported once per next hop.
	assert.Equal(t, "198.51.100.128/28", routes[2].Prefix)
	assert.Equal(t, "198.51.100.133", routes[2].NextHopIP)
	assert.Equal(t, "198.51.100.134", routes[3].NextHopIP)
	assert.True(t, routes[2].Best)
	assert.True(t, routes[3].Best)
}
