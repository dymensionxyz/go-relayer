package relayer

import (
	"context"
	"errors"
	"time"

	"github.com/cosmos/relayer/v2/relayer/provider"
)

func SendGenesisTransfer(
	ctx context.Context,
	hubC *Chain,
	raC *Chain,
	maxRetries uint64,
	timeout time.Duration,
	srcPortID, dstPortID, order, version string,
	override bool,
	memo string,
	pathName string,
	channelID string,
) error {

	hub, ok := hubC.
	ra, ok := raC.ChainProvider.(provider.RollappProvider)
	if !ok {
		return errors.New("not rollapp provider")
	}
	return ra.TrySendGenesisTransfer(ctx, channelID)
}
