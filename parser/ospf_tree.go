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

// lookupRoot returns the block for an area/identity pair, creating it once.
// BIRD can name the same root twice, so the blocks are merged instead of being
// collected twice: a second series with identical labels would abort the whole
// Prometheus scrape.
func lookupRoot(roots *[]*protocol.OSPFSpfRoot, byKey map[string]*protocol.OSPFSpfRoot,
	area, kind, identity string,
) *protocol.OSPFSpfRoot {
	key := area + "\x00" + kind + "\x00" + identity
	if root, seen := byKey[key]; seen {
		return root
	}

	root := &protocol.OSPFSpfRoot{Area: area, Kind: kind, Identity: identity}
	byKey[key] = root
	*roots = append(*roots, root)

	return root
}

// contains reports whether a slice already holds a value.
func contains(values []string, value string) bool {
	for _, current := range values {
		if current == value {
			return true
		}
	}

	return false
}

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
	rootsByKey := map[string]*protocol.OSPFSpfRoot{}
	distanceOf := map[*protocol.OSPFSpfRoot]bool{}
	area := ""
	var current *protocol.OSPFSpfRoot
	rootLevel := -1

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
			rootLevel = -1
			continue
		}

		// A block header is a line at the indentation the first block of the
		// area uses. Anything deeper belongs to the block that is open, so the
		// metric-less "router <id>" membership lines of a transit network do not
		// open a second root, no matter how far BIRD indents them.
		if m := ospfTreeRoot.FindStringSubmatch(line); m != nil && (current == nil || level <= rootLevel) {
			current = lookupRoot(&roots, rootsByKey, area, m[1], m[2])
			rootLevel = level
			continue
		}

		if current == nil {
			continue
		}

		if m := ospfTreeDistance.FindStringSubmatch(line); m != nil {
			if _, seen := distanceOf[current]; !seen {
				distanceOf[current] = true
				current.Distance = parseInt(m[1])
			}

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
			if kind == "router" && current.Kind == "network" && !contains(current.Routers, identity) {
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
