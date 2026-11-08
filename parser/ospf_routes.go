package parser

import (
	"bufio"
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"github.com/czerwonk/bird_exporter/protocol"
)

var (
	ospfRouteHeader   = regexp.MustCompile(`^(\S+)\s+unicast\s+\[([^\]]*)\]\s+(.+)$`)
	ospfRouteMetric   = regexp.MustCompile(`\((\d+)/(\d+)\)`)
	ospfRouteRouterID = regexp.MustCompile(`\[([^\]]+)\]\s*$`)
	ospfRouteVia      = regexp.MustCompile(`^via (\S+) on (\S+)$`)
	ospfRouteDev      = regexp.MustCompile(`^dev (\S+)$`)
	ospfRouteAttr     = regexp.MustCompile(`^([A-Za-z][A-Za-z0-9_.]*):\s*(.*)$`)
)

type pendingRoute struct {
	prefix     string
	best       bool
	routeType  string
	metric1    int64
	metric2    int64
	routerID   string
	nexthops   []protocol.OSPFRoute
	hasNexthop bool
}

// ParseOSPFRoutes parses the route blocks of "show route all protocol <name>".
//
// Each route is returned once per next hop, so ECMP routes yield one entry per
// path. Routes with only a device next hop carry an empty NextHopIP.
func ParseOSPFRoutes(data []byte) []*protocol.OSPFRoute {
	routes, _ := ParseOSPFRoutesWithError(data)
	return routes
}

// ParseOSPFRoutesWithError parses the route blocks and reports scanner failures.
func ParseOSPFRoutesWithError(data []byte) ([]*protocol.OSPFRoute, error) {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 64<<10), maxProtocolLineBytes)

	routes := make([]*protocol.OSPFRoute, 0)
	var current *pendingRoute

	for scanner.Scan() {
		line := trimmedLine(scanner.Text())
		if line == "" {
			continue
		}

		if m := ospfRouteHeader.FindStringSubmatch(line); m != nil {
			if current != nil {
				routes = append(routes, current.expand()...)
			}

			rest := m[3]
			current = &pendingRoute{prefix: m[1], routeType: "OSPF"}

			for _, field := range strings.Fields(rest) {
				if field == "*" {
					current.best = true
				}
			}

			if mm := ospfRouteMetric.FindStringSubmatch(rest); mm != nil {
				current.metric1 = parseInt(mm[2])
			}

			if mm := ospfRouteRouterID.FindStringSubmatch(rest); mm != nil {
				current.routerID = mm[1]
			}

			continue
		}

		if current == nil {
			continue
		}

		if m := ospfRouteVia.FindStringSubmatch(line); m != nil {
			current.hasNexthop = true
			current.nexthops = append(current.nexthops, protocol.OSPFRoute{
				Prefix:    current.prefix,
				NextHopIP: m[1],
				Interface: m[2],
			})

			continue
		}

		if m := ospfRouteDev.FindStringSubmatch(line); m != nil {
			current.hasNexthop = true
			current.nexthops = append(current.nexthops, protocol.OSPFRoute{
				Prefix:    current.prefix,
				Interface: m[1],
			})

			continue
		}

		m := ospfRouteAttr.FindStringSubmatch(line)
		if m == nil {
			continue
		}

		switch m[1] {
		case "Type":
			current.routeType = m[2]
		case "OSPF.metric1":
			current.metric1 = parseInt(m[2])
		case "OSPF.metric2":
			current.metric2 = parseInt(m[2])
		case "OSPF.router_id":
			current.routerID = m[2]
		}
	}

	if current != nil {
		routes = append(routes, current.expand()...)
	}

	if err := scanner.Err(); err != nil {
		return routes, fmt.Errorf("scan BIRD route reply: %w", err)
	}

	return routes, nil
}

func (p *pendingRoute) expand() []*protocol.OSPFRoute {
	routes := make([]*protocol.OSPFRoute, 0, len(p.nexthops))

	for i := range p.nexthops {
		route := p.nexthops[i]
		route.AdvertisedBy = p.routerID
		route.Metric1 = p.metric1
		route.Metric2 = p.metric2
		route.Type = p.routeType
		route.Best = p.best
		routes = append(routes, &route)
	}

	if !p.hasNexthop {
		routes = append(routes, &protocol.OSPFRoute{
			Prefix:       p.prefix,
			AdvertisedBy: p.routerID,
			Metric1:      p.metric1,
			Metric2:      p.metric2,
			Type:         p.routeType,
			Best:         p.best,
		})
	}

	return routes
}
