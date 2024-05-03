package cosmos

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/cosmos/gogoproto/sortkeys"

	"github.com/cosmos/relayer/v2/relayer/processor"
)

func getStuckPackets(sp *processor.StuckPacket) ([]*processor.StuckPacket, error) {
	if sp == nil {
		return nil, nil
	}
	j := sp.ChainID
	parts := strings.Split(j, "#")
	if len(parts) != 2 {
		return []*processor.StuckPacket{sp}, nil
	}
	return getStuckPacketsFromFile(parts[0], parts[1])
}

func getStuckPacketsFromFile(chainID string, fn string) ([]*processor.StuckPacket, error) {
	file, err := os.Open(fn)
	if err != nil {
		return nil, err
	}
	var x []uint64
	// err = json.Unmarshal(file, &x)
	p := json.NewDecoder(file)
	if err = p.Decode(&x); err != nil {
		return nil, err
	}
	sortkeys.Uint64s(x)
	ret := make([]*processor.StuckPacket, 0, len(x))
	for _, h := range x {
		ret = append(ret, &processor.StuckPacket{
			ChainID:     chainID,
			StartHeight: h,
			EndHeight:   h,
		})
	}

	return ret, nil
}
