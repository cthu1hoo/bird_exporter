package protocol

// OSPFNeighbor is one row of "show ospf neighbors".
//
// State is the OSPF finite state machine (Down, Attempt, Init, 2-Way, ExStart,
// Exchange, Loading, Full). Role is the network role reported in the same
// column (PtP, DR, BDR, DROther). HoldTimer is the remaining hold (dead) time
// in seconds; BIRD refreshes it to the interface dead timer on every hello and
// removes the neighbor entry when it reaches zero.
type OSPFNeighbor struct {
	RouterID   string
	Priority   int64
	State      string
	Role       string
	HoldTimer  float64
	Interface  string
	NeighborIP string
	Area       string
}

// Adjacent reports whether the neighbor has reached the Full state.
func (n *OSPFNeighbor) Adjacent() bool {
	return n.State == "Full"
}

// OSPFInterface is one block of "show ospf interface".
//
// Network is the interface network in prefix notation. State may carry a
// "(stub)" suffix, which is reported through Stub instead.
type OSPFInterface struct {
	Name       string
	Network    string
	Type       string
	Area       string
	AreaID     string
	State      string
	Stub       bool
	Priority   int64
	Cost       int64
	ECMPWeight int64
	HelloTimer float64
	WaitTimer  float64
	DeadTimer  float64
	Retransmit float64
	DRID       string
	DRIp       string
	BDRID      string
	BDRIp      string
}

// OSPFLSA is one row of "show ospf lsadb".
//
// Area is the area ID for per-area tables and the empty string for the "Global"
// table (AS-external LSAs). Type is the LSA type as printed by BIRD, e.g.
// "0001" for a router LSA.
type OSPFLSA struct {
	Area     string
	Type     string
	LSID     string
	Router   string
	Sequence string
	Age      int64
	Checksum string
}

// OSPFSpfNode is one entry below a root in "show ospf state" / "show ospf topology".
//
// Kind is one of router, network, stubnet, xnetwork, xrouter, vlink or external.
// Metric is the total SPF cost from the root to this node. MetricType is "1" or
// "2" for external nodes (metric1/metric2) and empty otherwise.
type OSPFSpfNode struct {
	Kind       string
	Peer       string
	Prefix     string
	Metric     int64
	MetricType string
	Via        string
}

// OSPFSpfRoot is one "router <id>" block of the SPF output, including the
// network nodes printed at area level.
type OSPFSpfRoot struct {
	Area     string
	Kind     string
	Identity string
	Distance int64
	DR       string
	Routers  []string
	Nodes    []OSPFSpfNode
}

// OSPFRoute is one route of "show route all protocol <name>".
type OSPFRoute struct {
	Prefix       string
	AdvertisedBy string
	Metric1      int64
	Metric2      int64
	Type         string
	NextHopIP    string
	Interface    string
	Best         bool
}
