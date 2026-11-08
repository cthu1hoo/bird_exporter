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
	ospfInterface       = regexp.MustCompile(`^Interface (\S+) \((\S+)\)\s*$`)
	ospfInterfaceKV     = regexp.MustCompile(`^([A-Za-z][A-Za-z0-9 ()]*?):\s*(.*)$`)
	ospfInterfaceArea   = regexp.MustCompile(`^(\S+) \((\d+)\)\s*$`)
	ospfInterfaceState  = regexp.MustCompile(`^([^(]+)(?:\((\w+)\))?\s*$`)
	ospfInterfaceNumber = regexp.MustCompile(`^(\d+(?:\.\d+)?)\s*$`)
)

// ParseOSPFInterfaces parses the blocks of "show ospf interface".
func ParseOSPFInterfaces(data []byte) []*protocol.OSPFInterface {
	interfaces, _ := ParseOSPFInterfacesWithError(data)
	return interfaces
}

// ParseOSPFInterfacesWithError parses the blocks of "show ospf interface" and
// reports scanner failures.
func ParseOSPFInterfacesWithError(data []byte) ([]*protocol.OSPFInterface, error) {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 64<<10), maxProtocolLineBytes)

	interfaces := make([]*protocol.OSPFInterface, 0)
	var current *protocol.OSPFInterface

	for scanner.Scan() {
		line := trimmedLine(scanner.Text())

		if m := ospfInterface.FindStringSubmatch(line); m != nil {
			current = &protocol.OSPFInterface{Name: m[1], Network: m[2]}
			interfaces = append(interfaces, current)
			continue
		}

		if current == nil {
			continue
		}

		m := ospfInterfaceKV.FindStringSubmatch(line)
		if m == nil {
			continue
		}

		key := strings.TrimSpace(m[1])
		value := strings.TrimSpace(m[2])

		switch key {
		case "Type":
			current.Type = value
		case "Area":
			if am := ospfInterfaceArea.FindStringSubmatch(value); am != nil {
				current.Area = am[1]
				current.AreaID = am[2]
			} else {
				current.Area = value
			}
		case "State":
			if sm := ospfInterfaceState.FindStringSubmatch(value); sm != nil {
				current.State = strings.TrimSpace(sm[1])
				current.Stub = sm[2] == "stub"
			} else {
				current.State = value
			}
		case "Priority":
			current.Priority = parseInt(number(value))
		case "Cost":
			current.Cost = parseInt(number(value))
		case "ECMP weight":
			current.ECMPWeight = parseInt(number(value))
		case "Hello timer":
			current.HelloTimer = parseFloat(number(value))
		case "Wait timer":
			current.WaitTimer = parseFloat(number(value))
		case "Dead timer":
			current.DeadTimer = parseFloat(number(value))
		case "Retransmit timer":
			current.Retransmit = parseFloat(number(value))
		case "Designated router (ID)":
			current.DRID = electedRouterID(value)
		case "Designated router (IP)":
			current.DRIp = electedRouterID(value)
		case "Backup designated router (ID)":
			current.BDRID = electedRouterID(value)
		case "Backup designated router (IP)":
			current.BDRIp = electedRouterID(value)
		}
	}

	if err := scanner.Err(); err != nil {
		return interfaces, fmt.Errorf("scan BIRD OSPF interface reply: %w", err)
	}

	return interfaces, nil
}

// electedRouterID drops the 0.0.0.0 placeholder BIRD prints while no designated
// router was elected, so callers can treat an empty value as "no DR".
func electedRouterID(value string) string {
	if value == "0.0.0.0" {
		return ""
	}

	return value
}

// number keeps only the numeric part of a value such as "1" or "1 (default)".
func number(value string) string {
	m := ospfInterfaceNumber.FindStringSubmatch(value)
	if m != nil {
		return m[1]
	}

	fields := strings.Fields(value)
	if len(fields) == 0 {
		return ""
	}

	return fields[0]
}
