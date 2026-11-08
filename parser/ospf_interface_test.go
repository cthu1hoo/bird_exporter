package parser

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// interfacesLive is a trimmed "show ospf interface" reply from a production
// router, with addresses replaced by documentation ranges.
const interfacesLive = "ospf1:\n" +
	"Interface eth1 (198.51.100.128/28)\n" +
	"\tType: broadcast\n" +
	"\tArea: 0.0.0.0 (0)\n" +
	"\tState: DR\n" +
	"\tPriority: 1\n" +
	"\tCost: 10\n" +
	"\tECMP weight: 1\n" +
	"\tHello timer: 5\n" +
	"\tWait timer: 10\n" +
	"\tDead timer: 20\n" +
	"\tRetransmit timer: 2\n" +
	"\tDesignated router (ID): 203.0.113.1\n" +
	"\tDesignated router (IP): 198.51.100.133\n" +
	"\tBackup designated router (ID): 203.0.113.2\n" +
	"\tBackup designated router (IP): 198.51.100.132\n" +
	"Interface wg_ck (192.0.2.8/30)\n" +
	"\tType: ptp\n" +
	"\tArea: 0.0.0.0 (0)\n" +
	"\tState: PtP\n" +
	"\tPriority: 1\n" +
	"\tCost: 15\n" +
	"\tECMP weight: 1\n" +
	"\tHello timer: 5\n" +
	"\tWait timer: 10\n" +
	"\tDead timer: 20\n" +
	"\tRetransmit timer: 2\n" +
	"Interface br_float (198.51.100.3/32)\n" +
	"\tType: broadcast\n" +
	"\tArea: 0.0.0.0 (0)\n" +
	"\tState: Waiting (stub)\n" +
	"\tPriority: 1\n" +
	"\tCost: 10\n" +
	"\tECMP weight: 1\n" +
	"\tHello timer: 10\n" +
	"\tWait timer: 40\n" +
	"\tDead timer: 40\n" +
	"\tRetransmit timer: 5\n" +
	"\tDesignated router (ID): 0.0.0.0\n" +
	"\tDesignated router (IP): 0.0.0.0\n" +
	"\tBackup designated router (ID): 0.0.0.0\n" +
	"\tBackup designated router (IP): 0.0.0.0\n"

func TestParseOSPFInterfaces(t *testing.T) {
	interfaces, err := ParseOSPFInterfacesWithError([]byte(interfacesLive))
	require.NoError(t, err)
	require.Len(t, interfaces, 3)

	backbone := interfaces[0]
	assert.Equal(t, "eth1", backbone.Name)
	assert.Equal(t, "198.51.100.128/28", backbone.Network)
	assert.Equal(t, "broadcast", backbone.Type)
	assert.Equal(t, "0.0.0.0", backbone.Area)
	assert.Equal(t, "0", backbone.AreaID)
	assert.Equal(t, "DR", backbone.State)
	assert.False(t, backbone.Stub)
	assert.Equal(t, int64(10), backbone.Cost)
	assert.InDelta(t, 5.0, backbone.HelloTimer, 0.001)
	assert.InDelta(t, 20.0, backbone.DeadTimer, 0.001)
	assert.Equal(t, "203.0.113.1", backbone.DRID)
	assert.Equal(t, "198.51.100.133", backbone.DRIp)
	assert.Equal(t, "203.0.113.2", backbone.BDRID)

	tunnel := interfaces[1]
	assert.Equal(t, "wg_ck", tunnel.Name)
	assert.Equal(t, "ptp", tunnel.Type)
	assert.Equal(t, "PtP", tunnel.State)
	assert.Equal(t, int64(15), tunnel.Cost)
	assert.False(t, tunnel.Stub)
	assert.Empty(t, tunnel.DRID)

	stub := interfaces[2]
	assert.Equal(t, "br_float", stub.Name)
	assert.Equal(t, "Waiting", stub.State)
	assert.True(t, stub.Stub)
	assert.InDelta(t, 40.0, stub.DeadTimer, 0.001)

	// BIRD prints 0.0.0.0 while no designated router was elected.
	assert.Empty(t, stub.DRID)
	assert.Empty(t, stub.DRIp)
	assert.Empty(t, stub.BDRID)
	assert.Empty(t, stub.BDRIp)
}
