package parser

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// neighborsLive is the "show ospf neighbors" reply of a production router
// (BIRD 2.17.5), with addresses replaced by RFC 5737/5738 documentation ranges.
const neighborsLive = "ospf1:\n" +
	"Router ID   \tPri\t     State     \tDTime\tInterface  Router IP\n" +
	"203.0.113.2\t  1\tFull/BDR  \t18.398\teth1       198.51.100.132\n" +
	"198.51.100.5\t128\tFull/PtP  \t19.966\twg_home    192.0.2.25\n" +
	"203.0.113.3\t  1\tInit/PtP  \t18.216\twg_egw_eu1 192.0.2.54\n" +
	"203.0.113.4\t  1\tFull/PtP  \t18.696\twg_egw_eu2 192.0.2.30\n" +
	"198.51.100.6\t128\tFull/PtP  \t18.349\twg_ck      192.0.2.10\n"

func TestParseOSPFNeighbors(t *testing.T) {
	neighbors, err := ParseOSPFNeighborsWithError([]byte(neighborsLive))
	require.NoError(t, err)
	require.Len(t, neighbors, 5)

	backbone := neighbors[0]
	assert.Equal(t, "203.0.113.2", backbone.RouterID)
	assert.Equal(t, int64(1), backbone.Priority)
	assert.Equal(t, "Full", backbone.State)
	assert.Equal(t, "BDR", backbone.Role)
	assert.InDelta(t, 18.398, backbone.HoldTimer, 0.0001)
	assert.Equal(t, "eth1", backbone.Interface)
	assert.Equal(t, "198.51.100.132", backbone.NeighborIP)
	assert.True(t, backbone.Adjacent())

	flapping := neighbors[2]
	assert.Equal(t, "Init", flapping.State)
	assert.Equal(t, "PtP", flapping.Role)
	assert.False(t, flapping.Adjacent())

	for _, neighbor := range neighbors[1:] {
		assert.NotEmpty(t, neighbor.NeighborIP, "neighbor IP missing for %s", neighbor.Interface)
	}
}

// TestParseOSPFNeighborsReplyCodes covers the reply codes bird_socket passes
// through, for example "1002-ospf1:".
func TestParseOSPFNeighborsReplyCodes(t *testing.T) {
	data := "1002-ospf1:\n" +
		"1002-203.0.113.2\t  1\tFull/BDR  \t15.594\teth1       198.51.100.132\n" +
		"0000\n"

	neighbors, err := ParseOSPFNeighborsWithError([]byte(data))
	require.NoError(t, err)
	require.Len(t, neighbors, 1)
	assert.Equal(t, "eth1", neighbors[0].Interface)
	assert.InDelta(t, 15.594, neighbors[0].HoldTimer, 0.0001)
}

// TestParseOSPFNeighborsTerminalLayout covers the space padded layout an
// interactive birdc produces instead of tabs.
func TestParseOSPFNeighborsTerminalLayout(t *testing.T) {
	data := "Router ID       Pri          State      DTime   Interface  Router IP\n" +
		"203.0.113.2     1     Full/BDR        15.594  eth1       198.51.100.132\n" +
		"198.51.100.5   128     Full/PtP        17.170  wg_home    192.0.2.25\n"

	neighbors, err := ParseOSPFNeighborsWithError([]byte(data))
	require.NoError(t, err)
	require.Len(t, neighbors, 2)
	assert.Equal(t, "wg_home", neighbors[1].Interface)
	assert.Equal(t, "192.0.2.25", neighbors[1].NeighborIP)
}
