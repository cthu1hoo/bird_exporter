package parser

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// lsdbLive is the "show ospf lsadb" reply of a production router, trimmed and
// with addresses replaced by documentation ranges.
const lsdbLive = "\nGlobal\n\n" +
	" Type   LS ID           Router          Sequence   Age  Checksum\n" +
	" 0005  192.0.2.12      203.0.113.3     8000004e   299    3cfe\n" +
	" 0005  198.51.100.1    203.0.113.1     8000004e   303    b68d\n" +
	"\nArea 0.0.0.0\n\n" +
	" Type   LS ID           Router          Sequence   Age  Checksum\n" +
	" 0001  198.51.100.5    198.51.100.5    800000a6   281    088c\n" +
	" 0001  203.0.113.1     203.0.113.1     800019e5   284    baa8\n" +
	" 0002  198.51.100.133  203.0.113.1     8000019b  1390    3118\n"

func TestParseOSPFDatabase(t *testing.T) {
	lsas, err := ParseOSPFDatabaseWithError([]byte(lsdbLive))
	require.NoError(t, err)
	require.Len(t, lsas, 5)

	global := 0
	perArea := 0

	for _, lsa := range lsas {
		if lsa.Area == "" {
			global++
			assert.Equal(t, "0005", lsa.Type)
		} else {
			perArea++
			assert.Equal(t, "0.0.0.0", lsa.Area)
		}
	}

	assert.Equal(t, 2, global)
	assert.Equal(t, 3, perArea)

	router := lsas[3]
	assert.Equal(t, "0001", router.Type)
	assert.Equal(t, "203.0.113.1", router.LSID)
	assert.Equal(t, "203.0.113.1", router.Router)
	assert.Equal(t, "800019e5", router.Sequence)
	assert.Equal(t, int64(284), router.Age)
	assert.Equal(t, "baa8", router.Checksum)

	network := lsas[4]
	assert.Equal(t, "0002", network.Type)
	assert.Equal(t, "198.51.100.133", network.LSID)
	assert.Equal(t, int64(1390), network.Age)
}
