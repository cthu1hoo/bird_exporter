package client

import "github.com/czerwonk/bird_exporter/protocol"

// Client retrieves information from Bird routing daemon
type Client interface {

	// GetProtocols retrieves protocol information and statistics from bird
	GetProtocols() ([]*protocol.Protocol, error)

	// GetOSPFAreas retrieves OSPF specific information from bird
	GetOSPFAreas(protocol *protocol.Protocol) ([]*protocol.OSPFArea, error)

	// GetOSPFNeighbors retrieves the OSPF neighbor table from bird
	GetOSPFNeighbors(protocol *protocol.Protocol) ([]*protocol.OSPFNeighbor, error)

	// GetOSPFInterfaces retrieves the OSPF interface table from bird
	GetOSPFInterfaces(protocol *protocol.Protocol) ([]*protocol.OSPFInterface, error)

	// GetOSPFLSDB retrieves the OSPF link state database summary from bird
	GetOSPFLSDB(protocol *protocol.Protocol) ([]*protocol.OSPFLSA, error)

	// GetOSPFTopology retrieves the SPF calculation of an OSPF protocol from bird.
	// With all set, the tree of every router in the area is returned.
	GetOSPFTopology(protocol *protocol.Protocol, all bool) ([]*protocol.OSPFSpfRoot, error)

	// GetOSPFRoutes retrieves the routes learned by an OSPF protocol from bird
	GetOSPFRoutes(protocol *protocol.Protocol) ([]*protocol.OSPFRoute, error)

	// GetRouterID retrieves the router ID of the bird instance serving the
	// protocol's socket
	GetRouterID(protocol *protocol.Protocol) (string, error)

	// GetBFDSessions retrieves BFD specific information from bird
	GetBFDSessions(protocol *protocol.Protocol) ([]*protocol.BFDSession, error)
}
