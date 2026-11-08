package parser

import (
	"bufio"
	"bytes"
	"fmt"
	"regexp"

	"github.com/czerwonk/bird_exporter/protocol"
)

var (
	ospfLsdbArea   = regexp.MustCompile(`^Area (\S+)(?: \((\d+)\))?\s*$`)
	ospfLsdbGlobal = regexp.MustCompile(`^Global\s*$`)
	ospfLsdbRow    = regexp.MustCompile(`^([0-9A-Fa-f]{4})\s+(\S+)\s+(\S+)\s+([0-9A-Fa-f]+)\s+(\d+)\s+([0-9A-Fa-f]+)\s*$`)
)

// ParseOSPFDatabase parses the tables of "show ospf lsadb".
//
// Rows of the "Global" table get an empty Area, rows below an "Area <id>"
// heading get the area ID as printed (dotted quad for OSPFv2).
func ParseOSPFDatabase(data []byte) []*protocol.OSPFLSA {
	lsas, _ := ParseOSPFDatabaseWithError(data)
	return lsas
}

// ParseOSPFDatabaseWithError parses the tables of "show ospf lsadb" and reports
// scanner failures.
func ParseOSPFDatabaseWithError(data []byte) ([]*protocol.OSPFLSA, error) {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 64<<10), maxProtocolLineBytes)

	lsas := make([]*protocol.OSPFLSA, 0)
	area := ""

	for scanner.Scan() {
		line := trimmedLine(scanner.Text())

		if ospfLsdbGlobal.MatchString(line) {
			area = ""
			continue
		}

		if m := ospfLsdbArea.FindStringSubmatch(line); m != nil {
			area = m[1]
			continue
		}

		m := ospfLsdbRow.FindStringSubmatch(line)
		if m == nil {
			continue
		}

		lsas = append(lsas, &protocol.OSPFLSA{
			Area:     area,
			Type:     m[1],
			LSID:     m[2],
			Router:   m[3],
			Sequence: m[4],
			Age:      parseInt(m[5]),
			Checksum: m[6],
		})
	}

	if err := scanner.Err(); err != nil {
		return lsas, fmt.Errorf("scan BIRD OSPF lsadb reply: %w", err)
	}

	return lsas, nil
}
