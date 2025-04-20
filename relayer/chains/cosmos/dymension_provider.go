package cosmos

import (
	"context"
	"fmt"

	"github.com/avast/retry-go/v4"
	dymlightclienttypes "github.com/cosmos/relayer/v2/relayer/chains/cosmos/dym/lightclient/types"
	dymrollapptypes "github.com/cosmos/relayer/v2/relayer/chains/cosmos/dym/rollapp/types"
	rdktypes "github.com/cosmos/relayer/v2/relayer/chains/cosmos/rollapp/rdk/hub-genesis/types"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"github.com/dymensionxyz/gerr-cosmos/gerrc"
)

// GetLatestRollappStateHeight implements provider.DymensionHubProvider.
func (cc *CosmosProvider) GetLatestRollappStateHeight(ctx context.Context, rollappID string) (int64, error) {
	c := dymrollapptypes.NewQueryClient(cc)
	var res *dymrollapptypes.QueryGetLatestHeightResponse
	var err error
	if err = retry.Do(func() error {
		res, err = c.LatestHeight(ctx, &dymrollapptypes.QueryGetLatestHeightRequest{RollappId: rollappID})
		return err
	}, rtyAtt, rtyDel, rtyErr); err != nil {
		return 0, fmt.Errorf("query latest height: %w", err)
	}
	return int64(res.Height), nil
}

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

// query to rollapp
func (cc *CosmosProvider) ShouldSendGenesisTransfer(ctx context.Context) error {
	c := rdktypes.NewQueryClient(cc)
	var res *rdktypes.QueryStateResponse
	var err error
	if err = retry.Do(func() error {
		res, err = c.State(ctx, &rdktypes.QueryStateRequest{})
		return err
	}, rtyAtt, rtyDel, rtyErr); err != nil {
		return fmt.Errorf("query state: %w", err)
	}

	if res.State.OutboundTransfersEnabled {
		return gerrc.ErrCancelled.Wrap("not necessary: outbound transfers are already enabled")
	}
	if res.State.InFlight {
		return gerrc.ErrCancelled.Wrap("outbound transfer is in flight, try later")
	}
	return nil
}

// tx to rollapp
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
		return nil, gerrc.ErrUnknown
	}

	return res, nil
}
