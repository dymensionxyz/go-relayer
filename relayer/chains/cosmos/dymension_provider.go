package cosmos

import (
	"context"
	"fmt"

	dymlightclienttypes "github.com/cosmos/relayer/v2/relayer/chains/cosmos/dym/lightclient/types"
	dymrollapptypes "github.com/cosmos/relayer/v2/relayer/chains/cosmos/dym/rollapp/types"
	rdktypes "github.com/cosmos/relayer/v2/relayer/chains/cosmos/rollapp/rdk/hub-genesis/types"
	"github.com/cosmos/relayer/v2/relayer/provider"
)

/* -------------------------------------------------------------------------- */
/*                                   queries                                  */
/* -------------------------------------------------------------------------- */
// GetLatestRollappStateHeight queries the latest rollapp state height committed on the hub
// Querying to the hub chain
func (cc *CosmosProvider) GetLatestRollappStateHeight(ctx context.Context, rollappID string) (int64, error) {
	c := dymrollapptypes.NewQueryClient(cc)
	res, err := c.LatestHeight(ctx, &dymrollapptypes.QueryGetLatestHeightRequest{RollappId: rollappID})
	if err != nil {
		return 0, fmt.Errorf("query latest height: %w", err)
	}
	return int64(res.Height), nil
}

// GetCanonicalClient queries the canonical client ID for a rollapp
// Querying to the hub chain
func (cc *CosmosProvider) GetCanonicalClient(ctx context.Context, rollappID string) (string, error) {
	c := dymlightclienttypes.NewQueryClient(cc)
	res, err := c.LightClient(ctx, &dymlightclienttypes.QueryGetLightClientRequest{RollappId: rollappID})
	if err != nil {
		return "", fmt.Errorf("query: %w", err)
	}
	return res.ClientId, nil
}

// GetBridgeState queries the ROLLAPP for it's bridge state
// Querying to the rollapp chain
func (cc *CosmosProvider) GetBridgeState(ctx context.Context) (bool, error) {
	c := rdktypes.NewQueryClient(cc)
	res, err := c.State(ctx, &rdktypes.QueryStateRequest{})
	if err != nil {
		return false, fmt.Errorf("query state: %w", err)
	}

	return res.State.OutboundTransfersEnabled, nil
}

/* -------------------------------------------------------------------------- */
/*                                transactions                                */
/* -------------------------------------------------------------------------- */

// TrySetCanonicalClient attempts to set the canonical client ID for a rollapp on the hub chain.
// The canonical client can only be set once per rollapp.
// Broadcasts MsgSetCanonicalClient to the hub chain.
// Returns an error if the transaction fails.
func (cc *CosmosProvider) TrySetCanonicalClient(ctx context.Context, clientID string) error {
	signer, err := cc.Address()
	if err != nil {
		return fmt.Errorf("relayer bech32 wallet address: %w", err)
	}
	msg := &dymlightclienttypes.MsgSetCanonicalClient{
		ClientId: clientID,
		Signer:   signer,
	}

	return cc.simpleSend(ctx, msg, func(signer string) {
		msg.Signer = signer
	})
}

// TrySendGenesisTransfer attempts to send a genesis transfer message on the rollapp chain.
// Broadcasts MsgSendTransfer to the rollapp chain, which commits the genesis transfer to the rollapp state.
// Returns the transaction response if successful, or an error if the transaction fails.
func (cc *CosmosProvider) TrySendGenesisTransfer(ctx context.Context, channelID string) (*provider.RelayerTxResponse, error) {
	signer, err := cc.Address()
	if err != nil {
		return nil, fmt.Errorf("relayer bech32 wallet address: %w", err)
	}
	msg := &rdktypes.MsgSendTransfer{
		ChannelId: channelID,
		Signer:    signer,
	}

	res, ok, err := cc.SendMessage(ctx, NewCosmosMessage(msg, nil), "")
	if !ok || err != nil || res == nil {
		return nil, err
	}

	return res, nil
}
