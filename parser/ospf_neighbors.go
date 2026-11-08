package parser

import (
	"bufio"
	"bytes"
	"fmt"
	"regexp"

	"github.com/czerwonk/bird_exporter/protocol"
)

// ospfNeighbor matches one row of "show ospf neighbors":
//
//	109.68.212.12   1   Full/BDR   18.398   eth1    192.168.112.132
//
// The state column carries the network role after a slash. The address columns
// are kept generic so OSPFv3 (IPv6 router IDs and link addresses) also parses.
var ospfNeighbor = regexp.MustCompile(
	`^([0-9A-Fa-f:.]+)\s+(\d+)\s+([A-Za-z][A-Za-z0-9-]*)(?:/([A-Za-z][A-Za-z0-9-]*))?\s+` +
		`(\d+(?:\.\d+)?)\s+(\S+)\s+([0-9A-Fa-f:.]+)\s*$`,
)

// ParseOSPFNeighbors parses "show ospf neighbors" output.
func ParseOSPFNeighbors(data []byte) []*protocol.OSPFNeighbor {
	neighbors, _ := ParseOSPFNeighborsWithError(data)
	return neighbors
}

// ParseOSPFNeighborsWithError parses "show ospf neighbors" output and reports
// scanner failures.
func ParseOSPFNeighborsWithError(data []byte) ([]*protocol.OSPFNeighbor, error) {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 64<<10), maxProtocolLineBytes)

	neighbors := make([]*protocol.OSPFNeighbor, 0)

	for scanner.Scan() {
		line := trimmedLine(scanner.Text())
		m := ospfNeighbor.FindStringSubmatch(line)
		if m == nil {
			continue
		}

		neighbors = append(neighbors, &protocol.OSPFNeighbor{
			RouterID:   m[1],
			Priority:   parseInt(m[2]),
			State:      m[3],
			Role:       m[4],
			HoldTimer:  parseFloat(m[5]),
			Interface:  m[6],
			NeighborIP: m[7],
		})
	}

	if err := scanner.Err(); err != nil {
		return neighbors, fmt.Errorf("scan BIRD OSPF neighbor reply: %w", err)
	}

	return neighbors, nil
}
