package client

import (
	"fmt"

	"github.com/czerwonk/bird_exporter/parser"
	"github.com/czerwonk/bird_exporter/protocol"
	birdsocket "github.com/czerwonk/bird_socket"
)

// BirdClient communicates with the bird socket to retrieve information
type BirdClient struct {
	Options *BirdClientOptions
}

// BirdClientOptions defines options to connect to bird
type BirdClientOptions struct {
	BirdV2       bool
	BirdEnabled  bool
	Bird6Enabled bool
	BirdSocket   string
	Bird6Socket  string
}

// GetProtocols retrieves protocol information and statistics from bird
func (c *BirdClient) GetProtocols() ([]*protocol.Protocol, error) {
	ipVersions := make([]string, 0)
	if c.Options.BirdV2 {
		ipVersions = append(ipVersions, "")
	} else {
		if c.Options.BirdEnabled {
			ipVersions = append(ipVersions, "4")
		}

		if c.Options.Bird6Enabled {
			ipVersions = append(ipVersions, "6")
		}
	}

	return c.protocolsFromBird(ipVersions)
}

// GetOSPFAreas retrieves OSPF specific information from bird
func (c *BirdClient) GetOSPFAreas(protocol *protocol.Protocol) ([]*protocol.OSPFArea, error) {
	sock := c.socketFor(protocol.IPVersion)
	b, err := birdsocket.Query(sock, fmt.Sprintf("show ospf %s", protocol.Name))
	if err != nil {
		return nil, err
	}

	return parser.ParseOSPFWithError(b)
}

// GetOSPFNeighbors retrieves the OSPF neighbor table from bird
func (c *BirdClient) GetOSPFNeighbors(p *protocol.Protocol) ([]*protocol.OSPFNeighbor, error) {
	b, err := c.queryOSPF("show ospf neighbors", p)
	if err != nil {
		return nil, err
	}

	return parser.ParseOSPFNeighborsWithError(b)
}

// GetOSPFInterfaces retrieves the OSPF interface table from bird
func (c *BirdClient) GetOSPFInterfaces(p *protocol.Protocol) ([]*protocol.OSPFInterface, error) {
	b, err := c.queryOSPF("show ospf interface", p)
	if err != nil {
		return nil, err
	}

	return parser.ParseOSPFInterfacesWithError(b)
}

// GetOSPFLSDB retrieves the OSPF link state database summary from bird
func (c *BirdClient) GetOSPFLSDB(p *protocol.Protocol) ([]*protocol.OSPFLSA, error) {
	b, err := c.queryOSPF("show ospf lsadb", p)
	if err != nil {
		return nil, err
	}

	return parser.ParseOSPFDatabaseWithError(b)
}

// GetOSPFTopology retrieves the SPF calculation of an OSPF protocol from bird
func (c *BirdClient) GetOSPFTopology(p *protocol.Protocol, all bool) ([]*protocol.OSPFSpfRoot, error) {
	command := "show ospf topology"
	if all {
		command = "show ospf topology all"
	}

	b, err := c.queryOSPF(command, p)
	if err != nil {
		return nil, err
	}

	return parser.ParseOSPFTopologyWithError(b)
}

// GetOSPFRoutes retrieves the routes learned by an OSPF protocol from bird
func (c *BirdClient) GetOSPFRoutes(p *protocol.Protocol) ([]*protocol.OSPFRoute, error) {
	sock := c.socketFor(p.IPVersion)
	b, err := birdsocket.Query(sock, fmt.Sprintf("show route all protocol %s", p.Name))
	if err != nil {
		return nil, err
	}

	return parser.ParseOSPFRoutesWithError(b)
}

// GetRouterID retrieves the router ID of the bird instance serving the socket
func (c *BirdClient) GetRouterID(p *protocol.Protocol) (string, error) {
	s, err := c.StatusFromSocket(c.socketFor(p.IPVersion))
	if err != nil {
		return "", err
	}
	if s == nil {
		return "", fmt.Errorf("no status reply for socket %s", c.socketFor(p.IPVersion))
	}

	return s.RouterID, nil
}

// queryOSPF runs an "show ospf ..." command for a given protocol instance. The
// instance name goes after the subcommand: "show ospf neighbors ospf1".
func (c *BirdClient) queryOSPF(command string, p *protocol.Protocol) ([]byte, error) {
	sock := c.socketFor(p.IPVersion)

	return birdsocket.Query(sock, fmt.Sprintf("%s %s", command, p.Name))
}

// GetBFDSessions retrieves BFD specific information from bird
func (c *BirdClient) GetBFDSessions(protocol *protocol.Protocol) ([]*protocol.BFDSession, error) {
	sock := c.socketFor(protocol.IPVersion)
	b, err := birdsocket.Query(sock, fmt.Sprintf("show bfd sessions %s", protocol.Name))
	if err != nil {
		return nil, err
	}

	return parser.ParseBFDSessionsWithError(protocol.Name, b)
}

func (c *BirdClient) protocolsFromBird(ipVersions []string) ([]*protocol.Protocol, error) {
	protocols := make([]*protocol.Protocol, 0)

	for _, ipVersion := range ipVersions {
		sock := c.socketFor(ipVersion)
		s, err := c.protocolsFromSocket(sock, ipVersion)
		if err != nil {
			return nil, err
		}

		protocols = append(protocols, s...)
	}

	return protocols, nil
}

func (c *BirdClient) protocolsFromSocket(socketPath string, ipVersion string) ([]*protocol.Protocol, error) {
	b, err := birdsocket.Query(socketPath, "show protocols all")
	if err != nil {
		return nil, err
	}

	return parser.ParseProtocolsWithError(b, ipVersion)
}

func (c *BirdClient) socketFor(ipVersion string) string {
	if !c.Options.BirdV2 && ipVersion == "6" {
		return c.Options.Bird6Socket
	}

	return c.Options.BirdSocket
}

// StatusFromSocket retrieves status information from bird
func (c *BirdClient) StatusFromSocket(socketPath string) (*parser.Status, error) {
	b, err := birdsocket.Query(socketPath, "show status")
	if err != nil {
		return nil, err
	}

	return parser.ParseStatusWithError(b)
}
