package parser

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStripReplyCode(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "protocol heading", input: "1002-ospf1:", want: "ospf1:"},
		{name: "indented continuation", input: "1004-\t\tOSPF.router_id: 203.0.113.1", want: "\t\tOSPF.router_id: 203.0.113.1"},
		{name: "no code", input: "\t\tdistance 10", want: "\t\tdistance 10"},
		{name: "digits in name", input: "peer0900 BGP master up", want: "peer0900 BGP master up"},
		{name: "terminal line", input: "0000", want: "0000"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, stripReplyCode(tt.input))
		})
	}
}

func TestIndentLevel(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{name: "no indent", input: "area 0.0.0.0", want: 0},
		{name: "one tab", input: "\trouter 203.0.113.1", want: 1},
		{name: "two tabs", input: "\t\tdistance 10", want: 2},
		{name: "spaces, eight per level", input: "                distance 10", want: 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, indentLevel(tt.input))
		})
	}
}

func TestParseOSPFNeighborsWithErrorRejectsOversizeLine(t *testing.T) {
	neighbors, err := ParseOSPFNeighborsWithError([]byte(strings.Repeat("x", maxProtocolLineBytes+1)))
	require.Error(t, err)
	require.Empty(t, neighbors)
}
