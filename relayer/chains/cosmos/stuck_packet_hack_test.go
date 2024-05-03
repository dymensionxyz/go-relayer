package cosmos

import (
	_ "embed"
	"testing"

	"github.com/cosmos/relayer/v2/relayer/processor"
	"github.com/stretchr/testify/require"
)

func TestStuckPacketHack(t *testing.T) {
	x := processor.StuckPacket{
		ChainID:     "nim_1122-1#/Users/danwt/Documents/dym/d-go-relayer/relayer/chains/cosmos/testdata/stuck.json",
		StartHeight: 0,
		EndHeight:   0,
	}

	sps, err := getStuckPackets(&x)
	require.NoError(t, err)
	t.Logf("Stuck Packets: %v\n", sps)
}
