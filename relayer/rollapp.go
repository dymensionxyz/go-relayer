package relayer

import (
	"context"
	"errors"
	"fmt"

	"github.com/cosmos/relayer/v2/relayer/provider"
)

func SendGenesisTransfer(
	ctx context.Context,
	hubC *Chain,
	raC *Chain,
) error {
	hub, ok := hubC.ChainProvider.(provider.DymensionHubProvider)
	if !ok {
		return errors.New("not dymension hub provider")
	}

	channelID, err := hub.GetCanonicalChan(ctx, raC.Chainid)
	if err != nil {
		return fmt.Errorf("get canonical chan: %w", err)
	}

	ra, ok := raC.ChainProvider.(provider.RollappProvider)
	if !ok {
		return errors.New("not rollapp provider")
	}
	return ra.TrySendGenesisTransfer(ctx, channelID)
}
