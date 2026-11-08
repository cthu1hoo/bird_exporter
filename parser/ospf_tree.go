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
	ospfTreeArea     = regexp.MustCompile(`^area (\S+)(?: \((\d+)\))?\s*$`)
	ospfTreeRoot     = regexp.MustCompile(`^(router|network)\s+(\S+)\s*$`)
	ospfTreeDistance = regexp.MustCompile(`^distance (\d+)\s*$`)
	ospfTreeDR       = regexp.MustCompile(`^dr (\S+)\s*$`)
	ospfTreeNode     = regexp.MustCompile(
		`^(router|network|stubnet|xnetwork|xrouter|vlink|external|ase)\s+(\S+)` +
			`(?:\s+metric(\d+)?\s+(\d+))?(?:\s+via\s+(\S+))?\s*$`,
	)
)

// ParseOSPFTopology parses the SPF output of "show ospf topology",
// "show ospf topology all", "show ospf state" and "show ospf state all".
//
// All four share the same shape: an "area <id>" heading, then one block per
// router (or per transit network) with an optional "distance", "dr" line and a
// list of nodes. The output of "state" additionally carries stubnet and
// external nodes.
func ParseOSPFTopology(data []byte) []*protocol.OSPFSpfRoot {
	roots, _ := ParseOSPFTopologyWithError(data)
	return roots
}

// ParseOSPFTopologyWithError parses the SPF output and reports scanner
// failures.
func ParseOSPFTopologyWithError(data []byte) ([]*protocol.OSPFSpfRoot, error) {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 64<<10), maxProtocolLineBytes)

	roots := make([]*protocol.OSPFSpfRoot, 0)
	area := ""
	var current *protocol.OSPFSpfRoot

	for scanner.Scan() {
		raw := stripReplyCode(scanner.Text())
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}

		level := indentLevel(raw)

		if m := ospfTreeArea.FindStringSubmatch(line); m != nil {
			area = m[1]
			current = nil
			continue
		}

		if m := ospfTreeRoot.FindStringSubmatch(line); m != nil && level <= 1 {
			current = &protocol.OSPFSpfRoot{Area: area, Kind: m[1], Identity: m[2]}
			roots = append(roots, current)
			continue
		}

		if current == nil {
			continue
		}

		if m := ospfTreeDistance.FindStringSubmatch(line); m != nil {
			current.Distance = parseInt(m[1])
			continue
		}

		if m := ospfTreeDR.FindStringSubmatch(line); m != nil {
			current.DR = m[1]
			continue
		}

		m := ospfTreeNode.FindStringSubmatch(line)
		if m == nil {
			continue
		}

		kind := m[1]
		identity := m[2]

		// Routers attached to a transit network are listed without a metric.
		if m[4] == "" {
			if kind == "router" && current.Kind == "network" {
				current.Routers = append(current.Routers, identity)
			}

			continue
		}

		node := protocol.OSPFSpfNode{
			Kind:       kind,
			Metric:     parseInt(m[4]),
			MetricType: m[3],
			Via:        m[5],
		}
		if strings.Contains(identity, "/") {
			node.Prefix = identity
		} else {
			node.Peer = identity
		}

		current.Nodes = append(current.Nodes, node)
	}

	if err := scanner.Err(); err != nil {
		return roots, fmt.Errorf("scan BIRD OSPF topology reply: %w", err)
	}

	return roots, nil
}
